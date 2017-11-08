package handlers_test

import (
	. "bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HttpJsonHandler", func() {

	var (
		handler            HTTPJSONHandler
		server             *ghttp.Server
		fakeLogger         *loggerfakes.FakeLogger
		fakeWriter         *internalfakes.FakeResponseWriter
		fakeServerResponse http.HandlerFunc
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
	})

	JustBeforeEach(func() {
		server = ghttp.NewUnstartedServer()
		server.AppendHandlers(fakeServerResponse)
		server.HTTPTestServer.Start()
		handler = NewHTTPJSONHandler(server.URL(), fakeLogger)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("successful requests", func() {
		BeforeEach(func() {
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/", "name=app-id.internal-domain.&type=28"),
				ghttp.RespondWith(http.StatusOK, `{
					"Status": 0,
					"TC": false,
					"RD": true,
					"RA": true,
					"AD": false,
					"CD": false,
					"Question":
					[
						{
							"name": "app-id.internal-domain.",
							"type": 28
						}
					],
					"Answer":
					[
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.1"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 28,
							"TTL": 224,
							"data": "::1"
						}
					],
					"Additional": [ ],
					"edns_client_subnet": "12.34.56.78/0"
				}`))
		})

		It("returns a DNS response based on answer given by backend server", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeAAAA)

			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).To(BeTrue())
			Expect(resp.Truncated).To(BeFalse())
			Expect(resp.Answer).To(HaveLen(2))
			Expect(resp.Answer[0]).To(Equal(&dns.A{
				Hdr: dns.RR_Header{
					Name:   "app-id.internal-domain.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    1526,
				},
				A: net.ParseIP("192.168.0.1"),
			}))

			Expect(resp.Answer[1]).To(Equal(&dns.A{
				Hdr: dns.RR_Header{
					Name:   "app-id.internal-domain.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    224,
				},
				A: net.ParseIP("::1"),
			}))
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				msg := &dns.Msg{}
				handler.ServeDNS(fakeWriter, msg)

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})
		})
	})

	Context("when it cannot reach the http server", func() {
		JustBeforeEach(func() {
			handler = NewHTTPJSONHandler("bogus-address", fakeLogger)
		})

		It("logs the error ", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			tag, template, args := fakeLogger.ErrorArgsForCall(0)
			Expect(tag).To(Equal("HTTPJSONHandler"))
			msg := fmt.Sprintf(template, args...)
			Expect(msg).To(ContainSubstring("error connecting to 'bogus-address': "))
			Expect(msg).To(ContainSubstring("Performing GET request"))
		})

		It("responds with a server fail", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeServerFailure))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).To(BeTrue())

			Expect(resp.Answer).To(HaveLen(0))
		})
	})

	Context("when it cannot write the response message", func() {
		BeforeEach(func() {
			fakeWriter.WriteMsgReturns(errors.New("failed to write message"))
		})

		It("logs the error", func() {
			handler.ServeDNS(fakeWriter, &dns.Msg{})

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
			Expect(tag).To(Equal("HTTPJSONHandler"))
			Expect(msg).To(Equal("failed to write message"))
		})
	})

	Context("when the http server response is malformed", func() {
		BeforeEach(func() {
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/"),
				ghttp.RespondWith(http.StatusOK, `{  garbage`),
			)
		})

		It("returns a serve fail response", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeServerFailure))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).To(BeTrue())

			Expect(resp.Answer).To(HaveLen(0))
		})

		It("logs the error", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			tag, template, args := fakeLogger.ErrorArgsForCall(0)
			Expect(tag).To(Equal("HTTPJSONHandler"))
			msg := fmt.Sprintf(template, args...)
			Expect(msg).To(ContainSubstring("failed to unmarshal response message"))
		})
	})

	Context("when the http server responds with non-200", func() {
		BeforeEach(func() {
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/"),
				ghttp.RespondWith(http.StatusNotFound, ""),
			)
		})

		It("returns a serve fail response", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeServerFailure))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).To(BeTrue())

			Expect(resp.Answer).To(HaveLen(0))
		})
	})

	Context("when the https server message is truncated", func() {
		BeforeEach(func() {
			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/", "name=app-id.internal-domain.&type=1"),
				ghttp.RespondWith(http.StatusOK, `{
					"Status": 0,
					"TC": true,
					"RD": true,
					"RA": true,
					"AD": false,
					"CD": false,
					"Question":
					[
						{
							"name": "app-id.internal-domain.",
							"type": 28
						}
					],
					"Answer":
					[
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.1"
						}
					],
					"Additional": [ ],
					"edns_client_subnet": "12.34.56.78/0"
				}`))
		})

		It("returns a truncated dns message", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(resp.Truncated).To(BeTrue())
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Answer).To(HaveLen(1))
		})
	})

	Context("when the non truncated http server response message is too large to fit in dns message", func() {
		BeforeEach(func() {
			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/", "name=app-id.internal-domain.&type=1"),
				ghttp.RespondWith(http.StatusOK, `{
					"Status": 0,
					"TC": false,
					"RD": true,
					"RA": true,
					"AD": false,
					"CD": false,
					"Question":
					[
						{
							"name": "app-id.internal-domain.",
							"type": 28
						}
					],
					"Answer": [
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.1"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.2"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 1526,
							"data": "192.168.0.3"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.4"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.5"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.6"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.7"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.8"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.9"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.10"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.11"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.12"
						},
						{
							"name": "app-id.internal-domain.",
							"type": 1,
							"TTL": 224,
							"data": "192.168.0.13"
						}
					],
					"Additional": [ ],
					"edns_client_subnet": "12.34.56.78/0"
				}`))
		})

		It("truncates the answers to fit", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(resp.RecursionAvailable).To(BeTrue())
			Expect(resp.Truncated).To(BeTrue())
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Answer).To(HaveLen(12))
		})
	})
})

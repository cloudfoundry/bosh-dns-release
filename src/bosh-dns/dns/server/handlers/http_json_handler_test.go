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

func startJsonServerWithHandler(handlerFunc http.HandlerFunc) *ghttp.Server {
	server := ghttp.NewUnstartedServer()
	server.AppendHandlers(handlerFunc)
	server.HTTPTestServer.Start()
	return server
}

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
		server = startJsonServerWithHandler(fakeServerResponse)
		handler = NewHTTPJSONHandler(server.URL(), fakeLogger)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("successful requets", func() {
		BeforeEach(func() {
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/ips/app-id.internal-domain."),
				ghttp.RespondWith(http.StatusOK, `{"ips":["192.168.0.1", "192.168.0.4"]}`),
			)
		})

		It("returns a DNS response based on answer given by backend server", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)

			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).ToNot(BeTrue())

			Expect(resp.Answer).To(HaveLen(2))
			Expect(resp.Answer[0]).To(Equal(&dns.A{
				Hdr: dns.RR_Header{
					Name:   "app-id.internal-domain.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.IPv4(192, 168, 0, 1),
			}))

			Expect(resp.Answer[1]).To(Equal(&dns.A{
				Hdr: dns.RR_Header{
					Name:   "app-id.internal-domain.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.IPv4(192, 168, 0, 4),
			}))
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				handler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})
	})

	Context("when it cannot reach the client", func() {
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
			Expect(msg).To(ContainSubstring("Error connecting to 'bogus-address': "))
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
			Expect(resp.RecursionAvailable).ToNot(BeTrue())

			Expect(resp.Answer).To(HaveLen(0))
		})
	})

	Context("when it cannot write the message", func() {
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

	Context("when the server responds with non-200", func() {
		BeforeEach(func() {
			fakeServerResponse = ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/ips/app-id.internal-domain."),
				ghttp.RespondWith(http.StatusNotFound, `{"ips":[]}`),
			)
		})

		It("Returns a serve fail response", func() {
			req := &dns.Msg{}
			req.SetQuestion("app-id.internal-domain.", dns.TypeA)
			handler.ServeDNS(fakeWriter, req)

			Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
			resp := fakeWriter.WriteMsgArgsForCall(0)
			Expect(resp.Question).To(Equal(req.Question))
			Expect(resp.Rcode).To(Equal(dns.RcodeServerFailure))
			Expect(resp.Authoritative).To(BeTrue())
			Expect(resp.RecursionAvailable).ToNot(BeTrue())

			Expect(resp.Answer).To(HaveLen(0))
		})
	})
})

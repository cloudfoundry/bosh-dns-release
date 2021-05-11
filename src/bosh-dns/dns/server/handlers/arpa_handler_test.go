package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"

	"github.com/miekg/dns"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArpaHandler", func() {
	Context("ServeDNS", func() {
		var (
			arpaHandler    handlers.ArpaHandler
			fakeWriter     *internalfakes.FakeResponseWriter
			fakeIPProvider *handlersfakes.FakeIPProvider
			fakeForwarder  *handlersfakes.FakeDNSHandler
			fakeLogger     *loggerfakes.FakeLogger
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeIPProvider = &handlersfakes.FakeIPProvider{}
			fakeForwarder = &handlersfakes.FakeDNSHandler{}

			arpaHandler = handlers.NewArpaHandler(fakeLogger, fakeIPProvider, fakeForwarder)
		})

		Context("when there are no questions", func() {
			It("responds with an rcode success", func() {
				arpaHandler.ServeDNS(fakeWriter, &dns.Msg{})
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when there are questions", func() {
			Describe("IPV4", func() {
				Context("and they are about external ips", func() {
					It("forwards the question up to a recursor", func() {
						m := &dns.Msg{}
						SetQuestion(m, nil, "109.22.25.104.in-addr.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeForwarder.ServeDNSCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsArgsForCall(0)).To(Equal("104.25.22.109"))
						forwardedWriter, forwardedMsg := fakeForwarder.ServeDNSArgsForCall(0)
						Expect(forwardedWriter).To(Equal(fakeWriter))
						Expect(forwardedMsg).To(Equal(m))
					})
				})

				Context("and they are about internal ips", func() {
					BeforeEach(func() {
						fakeIPProvider.GetFQDNsReturns([]string{"instance.fqdn", "index.fqdn"})
					})

					It("responds with an PTR records", func() {
						m := &dns.Msg{}
						SetQuestion(m, nil, "4.3.2.1.in-addr.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeIPProvider.GetFQDNsCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsArgsForCall(0)).To(Equal("1.2.3.4"))
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
						Expect(len(message.Answer)).To(Equal(2))
						Expect(message.Answer[0].(*dns.PTR).Ptr).To(Equal("instance.fqdn"))
						Expect(message.Answer[1].(*dns.PTR).Ptr).To(Equal("index.fqdn"))
					})
				})
			})

			Describe("IPV6", func() {

				Context("and they are about external ips", func() {
					It("forwards the question up to a recursor", func() {
						m := &dns.Msg{}
						SetQuestion(m, nil, "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeForwarder.ServeDNSCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsArgsForCall(0)).To(Equal("2001:4860:4860:0000:0000:0000:0000:0000"))
						forwardedWriter, forwardedMsg := fakeForwarder.ServeDNSArgsForCall(0)
						Expect(forwardedWriter).To(Equal(fakeWriter))
						Expect(forwardedMsg).To(Equal(m))
					})
				})

				Context("and they are about internal ips", func() {
					BeforeEach(func() {
						fakeIPProvider.GetFQDNsReturns([]string{"instance.fqdn", "index.fqdn"})
					})

					It("responds with an empty response", func() {
						m := &dns.Msg{}
						SetQuestion(m, nil, "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.a.e.d.f.e.e.b.4.3.2.1.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeIPProvider.GetFQDNsCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsArgsForCall(0)).To(Equal("1234:beef:dead:0000:0000:0000:0000:0000"))
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
						Expect(len(message.Answer)).To(Equal(2))
						Expect(message.Answer[0].(*dns.PTR).Ptr).To(Equal("instance.fqdn"))
						Expect(message.Answer[1].(*dns.PTR).Ptr).To(Equal("index.fqdn"))
					})

					It("fills in the zeroes", func() {
						m := &dns.Msg{}
						SetQuestion(m, nil, "0.d.a.e.d.f.e.e.b.4.3.2.1.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeIPProvider.GetFQDNsCallCount()).To(Equal(1))
						Expect(fakeIPProvider.GetFQDNsArgsForCall(0)).To(Equal("1234:beef:dead:0000:0000:0000:0000:0000"))
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
						Expect(len(message.Answer)).To(Equal(2))
						Expect(message.Answer[0].(*dns.PTR).Ptr).To(Equal("instance.fqdn"))
						Expect(message.Answer[1].(*dns.PTR).Ptr).To(Equal("index.fqdn"))
					})
				})
			})

			Describe("not a valid question", func() {
				It("responds with rcode failure", func() {
					m := &dns.Msg{}
					SetQuestion(m, nil, "wut.wut.wuuuuuuuuuuuuut.ip39.arpa", dns.TypePTR)

					arpaHandler.ServeDNS(fakeWriter, m)
					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeFormatError))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})
			})
		})
	})
})

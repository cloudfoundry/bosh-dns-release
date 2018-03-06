package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"

	"github.com/miekg/dns"

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
						m.SetQuestion("109.22.25.104.in-addr.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeForwarder.ServeDNSCallCount()).To(Equal(1))
						forwardedWriter, forwardedMsg := fakeForwarder.ServeDNSArgsForCall(0)
						Expect(forwardedWriter).To(Equal(fakeWriter))
						Expect(forwardedMsg).To(Equal(m))
					})
				})

				Context("and they are about internal ips", func() {
					BeforeEach(func() {
						fakeIPProvider.HasIPReturns(true)
					})

					It("responds with an rcode server failure", func() {
						m := &dns.Msg{}
						m.SetQuestion("4.3.2.1.in-addr.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeIPProvider.HasIPCallCount()).To(Equal(1))
						Expect(fakeIPProvider.HasIPArgsForCall(0)).To(Equal("1.2.3.4"))
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
					})
				})
			})

			Describe("IPV6", func() {
				BeforeEach(func() {
					fakeIPProvider.HasIPReturns(false)
				})

				Context("and they are about external ips", func() {
					It("forwards the question up to a recursor", func() {
						m := &dns.Msg{}
						m.SetQuestion("0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeForwarder.ServeDNSCallCount()).To(Equal(1))
						Expect(fakeIPProvider.HasIPCallCount()).To(Equal(1))
						forwardedWriter, forwardedMsg := fakeForwarder.ServeDNSArgsForCall(0)
						Expect(forwardedWriter).To(Equal(fakeWriter))
						Expect(forwardedMsg).To(Equal(m))
					})
				})

				Context("and they are about internal ips", func() {
					BeforeEach(func() {
						fakeIPProvider.HasIPReturns(true)
					})

					It("responds with an rcode server failure", func() {
						m := &dns.Msg{}
						m.SetQuestion("0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.a.e.d.f.e.e.b.4.3.2.1.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeIPProvider.HasIPCallCount()).To(Equal(1))
						Expect(fakeIPProvider.HasIPArgsForCall(0)).To(Equal("1234:beef:dead:0000:0000:0000:0000:0000"))
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
					})

					It("fills in the zeroes", func() {
						m := &dns.Msg{}
						m.SetQuestion("0.d.a.e.d.f.e.e.b.4.3.2.1.ip6.arpa.", dns.TypePTR)

						arpaHandler.ServeDNS(fakeWriter, m)
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))
					})
				})
			})

			Describe("not a valid question", func() {
				It("responds with rcode failure", func() {
					m := &dns.Msg{}
					m.SetQuestion("wut.wut.wuuuuuuuuuuuuut.ip39.arpa", dns.TypePTR)

					arpaHandler.ServeDNS(fakeWriter, m)
					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})
			})
		})
	})
})

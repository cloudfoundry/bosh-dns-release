package handlers_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiscoveryHandler", func() {
	Context("ServeDNS", func() {
		var (
			discoveryHandler handlers.DiscoveryHandler
			fakeWriter       *internalfakes.FakeResponseWriter
			fakeLogger       *loggerfakes.FakeLogger
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeLogger = &loggerfakes.FakeLogger{}

			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger)
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when there are questions", func() {
			It("returns rcode success for mx questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeMX)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode success for aaaa questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode server failure for all other questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("logging", func() {
			It("logs an error if the response fails to write", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("DiscoveryHandler"))
				Expect(msg).To(Equal("failed to write message"))
			})
		})
	})
})

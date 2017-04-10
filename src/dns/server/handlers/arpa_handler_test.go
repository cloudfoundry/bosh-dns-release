package handlers_test

import (
	"errors"

	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArpaHandler", func() {
	Context("ServeDNS", func() {
		var (
			arpaHandler handlers.ArpaHandler
			fakeWriter  *internalfakes.FakeResponseWriter
			fakeLogger  *loggerfakes.FakeLogger
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeWriter = &internalfakes.FakeResponseWriter{}

			arpaHandler = handlers.NewArpaHandler(fakeLogger)
		})

		Context("when there are no questions", func() {
			It("responds with an rcode success", func() {
				arpaHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when there are questions", func() {
			It("responds with an rcode server failure", func() {
				m := &dns.Msg{}
				m.SetQuestion("109.22.25.104.in-addr.arpa.", dns.TypePTR)

				arpaHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("logging", func() {
			It("logs the number of questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("109.22.25.104.in-addr.arpa.", dns.TypePTR)

				arpaHandler.ServeDNS(fakeWriter, m)
				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
				tag, msg, args := fakeLogger.InfoArgsForCall(0)
				Expect(tag).To(Equal("ArpaHandler"))
				Expect(msg).To(Equal("received a request with %d questions"))
				Expect(args[0]).To(Equal(1))
			})

			It("logs if there is an error during writing a message", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				arpaHandler.ServeDNS(fakeWriter, &dns.Msg{})

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("ArpaHandler"))
				Expect(msg).To(Equal("failed to write message"))
			})
		})
	})
})

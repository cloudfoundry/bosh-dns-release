package handlers_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"
)

var _ = Describe("DenyHandler", func() {
	var (
		denyHandler handlers.DenyHandler
		fakeWriter  *internalfakes.FakeResponseWriter
		fakeLogger  *loggerfakes.FakeLogger
		request     *dns.Msg
	)

	BeforeEach(func() {
		fakeWriter = &internalfakes.FakeResponseWriter{}
		fakeLogger = &loggerfakes.FakeLogger{}
		request = &dns.Msg{}
		request.SetQuestion("blocked.com.", dns.TypeA)
	})

	Context("when response type is NXDOMAIN", func() {
		BeforeEach(func() {
			denyHandler = handlers.NewDenyHandler("NXDOMAIN", fakeLogger)
		})

		It("returns NXDOMAIN", func() {
			denyHandler.ServeDNS(fakeWriter, request)

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeNameError))
			Expect(message.Authoritative).To(BeTrue())
			Expect(message.RecursionAvailable).To(BeFalse())
			Expect(message.Answer).To(BeEmpty())
		})
	})

	Context("when response type is REFUSED", func() {
		BeforeEach(func() {
			denyHandler = handlers.NewDenyHandler("REFUSED", fakeLogger)
		})

		It("returns REFUSED", func() {
			denyHandler.ServeDNS(fakeWriter, request)

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeRefused))
			Expect(message.Authoritative).To(BeTrue())
			Expect(message.RecursionAvailable).To(BeFalse())
			Expect(message.Answer).To(BeEmpty())
		})
	})

	Context("when response type is empty/default", func() {
		BeforeEach(func() {
			denyHandler = handlers.NewDenyHandler("", fakeLogger)
		})

		It("defaults to NXDOMAIN", func() {
			denyHandler.ServeDNS(fakeWriter, request)

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeNameError))
		})
	})

	Context("when WriteMsg returns an error", func() {
		BeforeEach(func() {
			denyHandler = handlers.NewDenyHandler("NXDOMAIN", fakeLogger)
			fakeWriter.WriteMsgReturns(errors.New("write failed"))
		})

		It("logs the error", func() {
			denyHandler.ServeDNS(fakeWriter, request)

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			tag, message, args := fakeLogger.ErrorArgsForCall(0)
			Expect(tag).To(Equal("DenyHandler"))
			Expect(message).To(ContainSubstring("error writing response"))
			Expect(args).To(ContainElement(ContainSubstring("write failed")))
		})
	})
})

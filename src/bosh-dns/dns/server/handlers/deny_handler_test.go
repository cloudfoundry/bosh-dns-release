package handlers_test

import (
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
		request     *dns.Msg
	)

	BeforeEach(func() {
		fakeWriter = &internalfakes.FakeResponseWriter{}
		request = &dns.Msg{}
		request.SetQuestion("blocked.com.", dns.TypeA)
	})

	Context("when response type is NXDOMAIN", func() {
		BeforeEach(func() {
			denyHandler = handlers.NewDenyHandler("NXDOMAIN")
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
			denyHandler = handlers.NewDenyHandler("REFUSED")
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
			denyHandler = handlers.NewDenyHandler("")
		})

		It("defaults to NXDOMAIN", func() {
			denyHandler.ServeDNS(fakeWriter, request)

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeNameError))
		})
	})
})

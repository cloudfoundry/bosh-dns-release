package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ bool = Describe("CacheHandler", func() {
	var (
		cacheHandler   handlers.CachingDNSHandler
		fakeWriter     *internalfakes.FakeResponseWriter
		fakeDnsHandler *handlersfakes.FakeDNSHandler
	)

	BeforeEach(func() {
		fakeDnsHandler = &handlersfakes.FakeDNSHandler{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		cacheHandler = handlers.NewCachingDNSHandler(fakeDnsHandler)
	})

	Describe("ServeDNS", func() {
		Context("when the request doesn't have recursion desired bit set", func() {
			It("forwards the question up to a recursor", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				m.RecursionDesired = false
				cacheHandler.ServeDNS(fakeWriter, m)
				Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
				forwardedWriter, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
				Expect(forwardedWriter).To(Equal(fakeWriter))
				Expect(forwardedMsg).To(Equal(m))
			})
		})
	})
})

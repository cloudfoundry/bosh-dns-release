package monitoring_test

import (
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/monitoring"

	"context"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNSRequestType", func() {
	var (
		fakeInternalDnsHandler handlersfakes.FakeDNSHandler
		fakeExternalDnsHandler handlersfakes.FakeDNSHandler

		fakeWriter internalfakes.FakeResponseWriter
		request    dns.Msg
	)

	BeforeEach(func() {
		fakeInternalDnsHandler = handlersfakes.FakeDNSHandler{}
		fakeExternalDnsHandler = handlersfakes.FakeDNSHandler{}

		fakeWriter = internalfakes.FakeResponseWriter{}
		request = dns.Msg{}
	})

	It("redirects internal dns requests", func() {
		pluginHandler := monitoring.NewPluginHandlerAdapter(&fakeInternalDnsHandler, &fakeExternalDnsHandler)

		internal := monitoring.NewRequestContext(monitoring.DNSRequestTypeInternal)
		pluginHandler.ServeDNS(internal, &fakeWriter, &request)

		Expect(fakeInternalDnsHandler.ServeDNSCallCount()).To(Equal(1))
		Expect(fakeExternalDnsHandler.ServeDNSCallCount()).To(Equal(0))
	})

	It("redirects external dns requests", func() {
		pluginHandler := monitoring.NewPluginHandlerAdapter(&fakeInternalDnsHandler, &fakeExternalDnsHandler)

		external := monitoring.NewRequestContext(monitoring.DNSRequestTypeExternal)
		pluginHandler.ServeDNS(external, &fakeWriter, &request)

		Expect(fakeInternalDnsHandler.ServeDNSCallCount()).To(Equal(0))
		Expect(fakeExternalDnsHandler.ServeDNSCallCount()).To(Equal(1))
	})

	It("redirects no dns requests without information in context", func() {
		pluginHandler := monitoring.NewPluginHandlerAdapter(&fakeInternalDnsHandler, &fakeExternalDnsHandler)

		pluginHandler.ServeDNS(context.Background(), &fakeWriter, &request)

		Expect(fakeInternalDnsHandler.ServeDNSCallCount()).To(Equal(0))
		Expect(fakeExternalDnsHandler.ServeDNSCallCount()).To(Equal(0))
	})

})

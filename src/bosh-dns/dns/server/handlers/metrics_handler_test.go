package handlers_test

import (
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/monitoring"
)

var _ bool = Describe("metricsHandler", func() {
	var (
		metricsHandler  handlers.MetricsDNSHandler
		metricsReporter monitoring.MetricsReporter
		metricsServer   monitoring.CoreDNSMetricsServer

		fakeWriter             *internalfakes.FakeResponseWriter
		fakeDnsHandlerInternal *handlersfakes.FakeDNSHandler
		fakeDnsHandlerExternal *handlersfakes.FakeDNSHandler
		request                *dns.Msg
	)

	BeforeEach(func() {
		fakeDnsHandlerInternal = &handlersfakes.FakeDNSHandler{}
		fakeDnsHandlerExternal = &handlersfakes.FakeDNSHandler{}
		fakeWriter = &internalfakes.FakeResponseWriter{}

		metricsServer = monitoring.MetricsServer("127.0.0.1:53088", fakeDnsHandlerInternal, fakeDnsHandlerExternal)
		fakeLogger := &loggerfakes.FakeLogger{}
		metricsReporter = monitoring.NewMetricsServerWrapper(fakeLogger, metricsServer).MetricsReporter()
		metricsHandler = handlers.NewMetricsDNSHandler(metricsReporter, monitoring.DNSRequestTypeExternal)
		request = &dns.Msg{}
	})

	Describe("ServeDNS", func() {
		It("calls DNS handler", func() {
			metricsHandler.ServeDNS(fakeWriter, request)

			Expect(fakeDnsHandlerInternal.ServeDNSCallCount()).To(Equal(0))
			Expect(fakeDnsHandlerExternal.ServeDNSCallCount()).To(Equal(1))
		})
	})
})

package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/monitoring/monitoringfakes"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ bool = Describe("metricsHandler", func() {
	var (
		metricsHandler      handlers.MetricsDNSHandler
		fakeMetricsReporter *monitoringfakes.FakeMetricsReporter
		fakeWriter          *internalfakes.FakeResponseWriter
		fakeDnsHandler      *handlersfakes.FakeDNSHandler
		response            *dns.Msg
	)

	BeforeEach(func() {
		fakeMetricsReporter = &monitoringfakes.FakeMetricsReporter{}
		fakeDnsHandler = &handlersfakes.FakeDNSHandler{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		metricsHandler = handlers.NewMetricsDNSHandler(fakeMetricsReporter, fakeDnsHandler)

	})

	Describe("ServeDNS", func() {
		It("collects metrics", func() {
			metricsHandler.ServeDNS(fakeWriter, response)

			Expect(fakeMetricsReporter.ReportCallCount()).To(Equal(1))
			Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
		})
	})
})

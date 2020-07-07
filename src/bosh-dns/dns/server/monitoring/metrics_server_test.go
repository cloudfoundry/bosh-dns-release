package monitoring_test

import (
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/monitoring"
	"bosh-dns/dns/server/monitoring/monitoringfakes"
	"context"
	"errors"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricsServerWrapper", func() {
	var (
		shutdown                 chan struct{}
		fakeLogger               *loggerfakes.FakeLogger
		fakeMetricsServerWrapper *monitoringfakes.FakeCoreDNSMetricsServer
	)

	Describe("Run", func() {

		BeforeEach(func() {
			shutdown = make(chan struct{})
			fakeMetricsServerWrapper = &monitoringfakes.FakeCoreDNSMetricsServer{}
			fakeLogger = &loggerfakes.FakeLogger{}
		})

		Context("when no errors happen", func() {
			BeforeEach(func() {
				fakeMetricsServerWrapper.OnStartupStub = func() error {
					close(shutdown)
					return nil
				}
			})

			It("starts and stops the server properly", func() {
				err := monitoring.NewMetricsServerWrapper(fakeLogger, fakeMetricsServerWrapper).Run(shutdown)

				Expect(err).ToNot(HaveOccurred())
				Expect(fakeMetricsServerWrapper.OnFinalShutdownCallCount()).To(Equal(1))
			})
		})

		Context("when start fails", func() {
			BeforeEach(func() {
				fakeMetricsServerWrapper.OnStartupStub = func() error {
					close(shutdown)
					return errors.New("")
				}
			})

			It("returns error and doesn't wait for stop", func() {
				err := monitoring.NewMetricsServerWrapper(fakeLogger, fakeMetricsServerWrapper).Run(shutdown)

				Expect(err).To(HaveOccurred())
				Expect(fakeMetricsServerWrapper.OnFinalShutdownCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Report", func() {
		var (
			metritcsReporter monitoring.MetricsReporter
			metricsServer    monitoring.CoreDNSMetricsServer
			fakeWriter       *internalfakes.FakeResponseWriter
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			metricsServer = monitoring.MetricsServer("127.0.0.1:53088")
			metritcsReporter = monitoring.NewMetricsServerWrapper(fakeLogger, metricsServer).MetricsReporter()
		})

		It("collects metrics", func() {
			m := &dns.Msg{}

			metritcsReporter.Report(context.Background(), fakeWriter, m)

			Expect(findMetric(metricsServer, "coredns_dns_requests_total")).To(Equal(1.0))
			Expect(findMetric(metricsServer, "coredns_dns_responses_total")).To(Equal(1.0))
		})
	})
})

func findMetric(metricsServer monitoring.CoreDNSMetricsServer, key string) float64 {
	metricFamilies, _ := metricsServer.(*metrics.Metrics).Reg.Gather()
	for _, mf := range metricFamilies {
		if mf.GetName() == key {
			return *mf.GetMetric()[0].Counter.Value
		}
	}
	return -1.0
}

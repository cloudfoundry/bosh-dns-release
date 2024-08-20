package performance_test

import (
	"bosh-dns/dns/server/records/recordsfakes"
	"fmt"
	"time"

	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/records"
	zp "bosh-dns/performance_tests/zone_pickers"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS", func() {
	var (
		picker zp.ZonePicker
		label  string

		dnsServerAddress  = "127.0.0.2:9953"
		durationInSeconds = 60 * 10
		workers           = 10
		requestsPerSecond = 7
	)

	BeforeEach(func() {
		setupServers()
	})

	AfterEach(func() {
		shutdownServers()
	})

	TestDNSPerformance := func(context string, timeThresholds TimeThresholds, vitalsThresholds VitalsThresholds) {
		PerformanceTest{
			Application:       "dns",
			Context:           context,
			Workers:           workers,
			RequestsPerSecond: requestsPerSecond,

			ServerPID: dnsSession.Command.Process.Pid,

			TimeThresholds:   timeThresholds,
			VitalsThresholds: vitalsThresholds,
			SuccessStatus:    dns.RcodeSuccess,

			WorkerFunc: func(resultChan chan<- Result) {
				MakeDNSRequestUntilSuccessful(picker, dnsServerAddress, resultChan)
			},
		}.Setup().TestPerformance(durationInSeconds, label)
	}

	Describe("using zones from file", func() {
		BeforeEach(func() {
			var err error
			picker, err = zp.NewZoneFilePickerFromFile("assets/zones.json")
			Expect(err).ToNot(HaveOccurred())
			label = "prod-like zones"
		})

		It("handles DNS responses quickly for prod like zones", func() {
			numberOfMeasurements := durationInSeconds * 2

			cpuSample := metrics.NewExpDecaySample(numberOfMeasurements, 0.015)
			cpuHistogram := metrics.NewHistogram(cpuSample)
			metrics.Register("CPU Usage", cpuHistogram) //nolint:errcheck

			memSample := metrics.NewExpDecaySample(numberOfMeasurements, 0.015)
			memHistogram := metrics.NewHistogram(memSample)
			metrics.Register("Mem Usage", memHistogram) //nolint:errcheck

			benchmarkTime := generateTimeHistogram(
				PerformanceTest{
					Application:       "dns-benchmark",
					Context:           "benchmark",
					Workers:           workers,
					RequestsPerSecond: requestsPerSecond,
					WorkerFunc: func(resultChan chan<- Result) {
						MakeDNSRequestUntilSuccessful(picker, "34.194.75.123:53", resultChan)
					},
				}.Setup().
					MakeParallelRequests(time.Duration(durationInSeconds)*time.Second, time.Second/2, cpuHistogram, memHistogram),
			)

			TestDNSPerformance("prod-like", TimeThresholdsFromBenchmark(benchmarkTime, 1.1), prodLikeVitalsThresholds())
		})
	})

	Describe("using upcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("upcheck.bosh-dns.")
			label = "upcheck zone"
		})

		It("handles DNS responses quickly for upcheck zone", func() {
			TestDNSPerformance("upcheck", upcheckTimeThresholds(), upcheckVitalsThresholds())
		})
	})

	Describe("using local bosh dns records", func() {
		var signal, shutdown chan struct{}

		BeforeEach(func() {
			signal = make(chan struct{})
			shutdown = make(chan struct{})

			logger := &fakes.FakeLogger{}
			healthWatcher := &healthinessfakes.FakeHealthWatcher{}
			fs := boshsys.NewOsFileSystem(logger)
			recordSetReader := records.NewFileReader("assets/records.json", fs, clock.NewClock(), logger, signal)
			fakeQueryFilterer := &recordsfakes.FakeFilterer{}
			fakeHealthFilterer := &recordsfakes.FakeFilterer{}
			fakeFiltererFactory := &recordsfakes.FakeFiltererFactory{}
			fakeFiltererFactory.NewQueryFiltererReturns(fakeQueryFilterer)
			fakeFiltererFactory.NewHealthFiltererReturns(fakeHealthFilterer)
			fakeAliasQueryEncoder := &recordsfakes.FakeAliasQueryEncoder{}
			recordSet, err := records.NewRecordSet(recordSetReader, aliases.NewConfig(), healthWatcher, uint(5), shutdown, logger, fakeFiltererFactory, fakeAliasQueryEncoder)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.AllRecords()).To(HaveLen(102))

			records := []string{}
			for _, record := range recordSet.AllRecords() {
				composed := fmt.Sprintf(
					"%s.%s.%s.%s.%s",
					record.ID,
					record.Group,
					record.Network,
					record.Deployment,
					record.Domain,
				)
				records = append(records, composed)
			}
			picker = &zp.ZoneFilePicker{Domains: records}
			label = "local zones"
		})

		AfterEach(func() {
			close(signal)
			close(shutdown)
		})

		It("handles DNS responses quickly for local zones", func() {
			TestDNSPerformance("local-zones", localZonesTimeThresholds(), localZonesVitalsThresholds())
		})
	})
})

func MakeDNSRequestUntilSuccessful(picker zp.ZonePicker, server string, result chan<- Result) {
	defer GinkgoRecover()
	zone := picker.NextZone()
	c := new(dns.Client)
	c.Timeout = 300 * time.Millisecond
	m := new(dns.Msg)

	startTime := time.Now()
	m.SetQuestion(dns.Fqdn(zone), dns.TypeA)

	for i := 0; i < 10; i++ {
		if i == 9 {
			c.Timeout = 3000 * time.Millisecond
		}
		r, _, err := c.Exchange(m, server)
		if err == nil {
			responseTime := time.Since(startTime)
			result <- Result{status: r.Rcode, time: time.Now().Unix(), metricName: "response_time", value: responseTime}
			return
		}
	}

	Fail(fmt.Sprintf("failed DNS request for %s via server %s after 10 retries", zone, server))
}

package performance_test

import (
	"time"

	zp "bosh-dns/performance_tests/zone_pickers"

	"bosh-dns/dns/server/records"

	"code.cloudfoundry.org/clock"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metrics "github.com/rcrowley/go-metrics"

	"fmt"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var _ = Describe("DNS", func() {
	var (
		picker zp.ZonePicker
		label  string

		dnsServerAddress  = "127.0.0.2:9953"
		durationInSeconds = 60 * 30
		workers           = 10
		requestsPerSecond = 7
	)

	TestDNSPerformance := func(context string, timeThresholds TimeThresholds) {
		PerformanceTest{
			Application:       "dns",
			Context:           context,
			Workers:           workers,
			RequestsPerSecond: requestsPerSecond,

			ServerPID: dnsSession.Command.Process.Pid,

			TimeThresholds: timeThresholds,
			VitalsThresholds: VitalsThresholds{
				CPUPct99: 25,
				MemPct99: 32,
			},

			SuccessStatus: dns.RcodeSuccess,

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
			duration := time.Duration(durationInSeconds) * time.Second
			resourcesInterval := time.Second / 2

			cpuSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
			cpuHistogram := metrics.NewHistogram(cpuSample)
			metrics.Register("CPU Usage", cpuHistogram)

			memSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
			memHistogram := metrics.NewHistogram(memSample)
			metrics.Register("Mem Usage", memHistogram)

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
					MakeParallelRequests(20*time.Second, resourcesInterval, cpuHistogram, memHistogram),
			)

			TestDNSPerformance("prod-like", TimeThresholdsFromBenchmark(benchmarkTime, 1.1))
		})
	})

	Describe("using upcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("upcheck.bosh-dns.")
			label = "upcheck zone"
		})

		It("handles DNS responses quickly for upcheck zone", func() {
			TestDNSPerformance("upcheck", TimeThresholds{
				Max:   7540 * time.Millisecond,
				Med:   1500 * time.Microsecond,
				Pct90: 4 * time.Millisecond,
				Pct95: 15 * time.Millisecond,
			})
		})
	})

	Describe("using local bosh dns records", func() {
		var (
			signal chan struct{}
		)

		BeforeEach(func() {
			signal = make(chan struct{})
			logger := &fakes.FakeLogger{}
			fs := boshsys.NewOsFileSystem(logger)
			recordSetReader := records.NewFileReader("assets/records.json", fs, clock.NewClock(), logger, signal)
			recordSet, err := records.NewRecordSet(recordSetReader, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).To(HaveLen(102))

			records := []string{}
			for _, record := range recordSet.Records {
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
		})

		It("handles DNS responses quickly for local zones", func() {
			TestDNSPerformance("local-zones", TimeThresholds{
				Max:   7540 * time.Millisecond,
				Med:   1500 * time.Microsecond,
				Pct90: 3 * time.Millisecond,
				Pct95: 15 * time.Millisecond,
			})
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

package performance_test

import (
	"time"

	zp "bosh-dns/performance_tests/zone_pickers"

	"bosh-dns/dns/server/records"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"io/ioutil"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
)

var _ = Describe("DNS", func() {
	var (
		picker zp.ZonePicker
		label  string

		dnsServerAddress  = "127.0.0.1:9953"
		durationInSeconds = 60 * 30
		workers           = 10
		requestsPerSecond = 7
	)

	TestDNSPerformance := func(medTimeThreshold time.Duration) {
		PerformanceTest{
			Workers:           workers,
			RequestsPerSecond: requestsPerSecond,

			MaxTimeThreshold: 7540 * time.Millisecond,
			MedTimeThreshold: medTimeThreshold,

			ServerPID: dnsSession.Command.Process.Pid,

			CPUThresholdMax:   20,
			CPUThresholdPct99: 5,
			MemThresholdMax:   25,

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
			benchmarkTime := generateTimeHistogram(
				PerformanceTest{
					Workers:           workers,
					RequestsPerSecond: requestsPerSecond,
					WorkerFunc: func(resultChan chan<- Result) {
						MakeDNSRequestUntilSuccessful(picker, "8.8.8.8:53", resultChan)
					},
				}.Setup().
					MakeParallelRequests(2 * time.Second),
			)

			maxLatency := benchmarkTime.Percentile(0.5)
			TestDNSPerformance(time.Duration(maxLatency) * time.Millisecond)
		})
	})

	Describe("using upcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("upcheck.bosh-dns.")
			label = "upcheck zone"
		})

		It("handles DNS responses quickly for upcheck zone", func() {
			TestDNSPerformance(1500 * time.Microsecond)
		})
	})

	Describe("using google zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("google.com.")
			label = "google.com zone"
		})

		It("handles DNS responses quickly for google zone", func() {
			benchmarkTime := generateTimeHistogram(
				PerformanceTest{
					Workers:           workers,
					RequestsPerSecond: requestsPerSecond,
					WorkerFunc: func(resultChan chan<- Result) {
						MakeDNSRequestUntilSuccessful(picker, "8.8.8.8:53", resultChan)
					},
				}.Setup().
					MakeParallelRequests(2 * time.Second),
			)

			maxLatency := benchmarkTime.Percentile(0.5)
			TestDNSPerformance(time.Duration(maxLatency) * time.Millisecond)
		})
	})

	Describe("using local bosh dns records", func() {
		BeforeEach(func() {
			logger := &fakes.FakeLogger{}
			recordsJsonBytes, err := ioutil.ReadFile("assets/records.json")
			Expect(err).ToNot(HaveOccurred())
			recordSet, err := records.CreateFromJSON(recordsJsonBytes, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).To(HaveLen(2))

			records := []string{}
			for _, record := range recordSet.Records {
				records = append(records, record.Fqdn(true))
			}
			picker = &zp.ZoneFilePicker{Domains: records}
			label = "local zones"
		})

		It("handles DNS responses quickly for local zones", func() {
			TestDNSPerformance(1500 * time.Microsecond)
		})
	})
})

func MakeDNSRequestUntilSuccessful(picker zp.ZonePicker, server string, result chan<- Result) {
	defer GinkgoRecover()
	zone := picker.NextZone()
	c := new(dns.Client)
	c.Timeout = 50 * time.Millisecond
	m := new(dns.Msg)

	startTime := time.Now()
	m.SetQuestion(dns.Fqdn(zone), dns.TypeA)

	for i := 0; i < 10; i++ {
		r, _, err := c.Exchange(m, server)
		if err == nil {
			responseTime := time.Since(startTime)
			result <- Result{status: r.Rcode, responseTime: responseTime}
			return
		}
	}

	Fail(fmt.Sprintf("failed DNS request for %s via server %s after 10 retries", zone, server))
}

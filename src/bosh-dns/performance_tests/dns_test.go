package performance_test

import (
	"time"

	zp "bosh-dns/performance_tests/zone_pickers"

	"bosh-dns/dns/server/records"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rcrowley/go-metrics"

	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Result struct {
	status       int
	responseTime time.Duration
}

var _ = Describe("DNS", func() {
	var (
		picker zp.ZonePicker
		label  string

		dnsServerAddress  = "127.0.0.1:9953"
		durationInSeconds = 60
		workers           = 10
		requestsPerSecond = 7
	)

	TestDNSPerformance := func(server string, requestsPerSecond int, durationInSeconds int, medianResponseBenchmark float64) {
		shutdown := make(chan struct{})

		duration := time.Duration(durationInSeconds) * time.Second
		resourcesInterval := time.Second

		cpuSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
		cpuHistogram := metrics.NewHistogram(cpuSample)
		metrics.Register("CPU Usage", cpuHistogram)

		memSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
		memHistogram := metrics.NewHistogram(memSample)
		metrics.Register("Mem Usage", memHistogram)

		done := make(chan struct{})
		go measureResourceUtilization(dnsSession.Command.Process.Pid, resourcesInterval, cpuHistogram, memHistogram, done, shutdown)

		results := makeParallelRequests(requestsPerSecond, workers, duration, shutdown, func(resultChan chan<- Result) {
			MakeDNSRequestUntilSuccessful(picker, server, resultChan)
		})
		<-done

		timeHistogram := generateTimeHistogram(results)

		successCount := 0
		for _, dr := range results {
			if dr.status == dns.RcodeSuccess {
				successCount++
			}
		}

		successPercentage := float64(successCount) / float64(len(results))
		fmt.Printf("success percentage is %.02f%%\n", successPercentage*100)
		fmt.Printf("requests per second is %d reqs/s\n", successCount/durationInSeconds)

		medTime := timeHistogram.Percentile(0.5) / float64(time.Millisecond)
		maxTime := timeHistogram.Max() / int64(time.Millisecond)
		printStatsForHistogram(timeHistogram, fmt.Sprintf("DNS handling latency for %s", label), "ms", 1000*1000)

		maxMem := float64(memHistogram.Max()) / (1024 * 1024)
		printStatsForHistogram(memHistogram, fmt.Sprintf("DNS server mem usage for %s", label), "MB", 1024*1024)

		maxCPU := float64(cpuHistogram.Max()) / (1000 * 1000)
		printStatsForHistogram(cpuHistogram, fmt.Sprintf("DNS server CPU usage for %s", label), "%", 1000*1000)

		testFailures := []error{}
		if (successCount / durationInSeconds) < requestsPerSecond {
			testFailures = append(testFailures, fmt.Errorf("Handled DNS requests %d per second was lower than %d benchmark", successCount/durationInSeconds, requestsPerSecond))
		}
		if successPercentage < 1 {
			testFailures = append(testFailures, fmt.Errorf("DNS success percentage of %.1f%% is too low", 100*successPercentage))
		}
		if medTime > medianResponseBenchmark {
			testFailures = append(testFailures, fmt.Errorf("Median DNS response time of %.3fms was greater than %.3fms benchmark", medTime, medianResponseBenchmark))
		}
		maxTimeinMsThreshold := int64(7540)
		if maxTime > maxTimeinMsThreshold {
			testFailures = append(testFailures, fmt.Errorf("Max DNS response time of %d.000ms was greater than %d.000ms benchmark", maxTime, maxTimeinMsThreshold))
		}
		cpuThresholdPercentage := float64(5)
		if maxCPU > cpuThresholdPercentage {
			testFailures = append(testFailures, fmt.Errorf("Max DNS server CPU usage of %.2f%% was greater than %.2f%% ceiling", maxCPU, cpuThresholdPercentage))
		}
		memThreshold := float64(15)
		if maxMem > memThreshold {
			testFailures = append(testFailures, fmt.Errorf("Max DNS server memory usage of %.2fMB was greater than %.2fMB ceiling", maxMem, memThreshold))
		}
		Expect(testFailures).To(BeEmpty())
	}

	Describe("using zones from file", func() {
		BeforeEach(func() {
			var err error
			picker, err = zp.NewZoneFilePickerFromFile("assets/zones.json")
			Expect(err).ToNot(HaveOccurred())
			label = "prod-like zones"
		})

		It("handles DNS responses quickly for prod like zones", func() {
			benchmarkTime := generateTimeHistogram(makeParallelRequests(requestsPerSecond, workers, 2*time.Second, make(chan struct{}), func(resultChan chan<- Result) {
				MakeDNSRequestUntilSuccessful(picker, "8.8.8.8:53", resultChan)
			}))

			maxLatency := benchmarkTime.Percentile(0.5)
			TestDNSPerformance(dnsServerAddress, requestsPerSecond, durationInSeconds, maxLatency)
		})
	})

	Describe("using upcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("upcheck.bosh-dns.")
			label = "upcheck zone"
		})

		It("handles DNS responses quickly for upcheck zone", func() {
			TestDNSPerformance(dnsServerAddress, requestsPerSecond, durationInSeconds, 1.5)
		})
	})

	Describe("using google zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("google.com.")
			label = "google.com zone"
		})

		It("handles DNS responses quickly for google zone", func() {
			benchmarkTime := generateTimeHistogram(makeParallelRequests(requestsPerSecond, workers, 2*time.Second, make(chan struct{}), func(resultChan chan<- Result) {
				MakeDNSRequestUntilSuccessful(picker, "8.8.8.8:53", resultChan)
			}))

			maxLatency := benchmarkTime.Percentile(0.5)
			TestDNSPerformance(dnsServerAddress, requestsPerSecond, durationInSeconds, maxLatency)
		})
	})

	Describe("using local bosh dns records", func() {
		BeforeEach(func() {
			recordsJsonBytes, err := ioutil.ReadFile("assets/records.json")
			Expect(err).ToNot(HaveOccurred())
			recordSet := records.RecordSet{}
			err = json.Unmarshal(recordsJsonBytes, &recordSet)
			Expect(err).ToNot(HaveOccurred())

			records := []string{}
			for _, record := range recordSet.Records {
				records = append(records, record.Fqdn(true))
			}
			picker = &zp.ZoneFilePicker{Domains: records}
			label = "local zones"
		})

		It("handles DNS responses quickly for local zones", func() {
			TestDNSPerformance(dnsServerAddress, requestsPerSecond, durationInSeconds, 1.5)
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

	Fail("failed DNS request after 10 retries")
}

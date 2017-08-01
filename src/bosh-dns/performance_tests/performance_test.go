package performance_test

import (
	"math"
	"sync"
	"time"

	zp "bosh-dns/performance_tests/zone_pickers"

	"github.com/cloudfoundry/gosigar"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rcrowley/go-metrics"

	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"bosh-dns/dns/server/records"
)

type PerformanceTestInfo struct {
	MedianRequestTime         time.Duration
	ErrorCount                int
	FailedRequestRCodesCounts map[int]int
	MaxRuntime                time.Duration
}

var _ = Describe("Performance", func() {
	var (
		maxDNSRequestsPerMin int
		info                 PerformanceTestInfo
		picker               zp.ZonePicker
		label                string
		dnsServerAddress     = "10.245.0.34:53"
		dnsServerPid         int
	)

	BeforeEach(func() {
		var found bool
		dnsServerPid, found = GetPidFor("dns")
		Expect(found).To(BeTrue())

		maxDNSRequestsPerMin = 420
		info = PerformanceTestInfo{}
	})

	TestDNSPerformance := func(server string) (metrics.Histogram, metrics.Histogram, metrics.Histogram, time.Duration) {
		flowSignal := createFlowSignal(10)
		wg, finishedDNSRequestsSignal := setupWaitGroupWithSignaler(maxDNSRequestsPerMin)
		result := make(chan DNSResult, maxDNSRequestsPerMin*2)

		startTime := time.Now()

		timeSample := metrics.NewExpDecaySample(maxDNSRequestsPerMin, 0.015)
		timeHistogram := metrics.NewHistogram(timeSample)
		metrics.Register("DNS response time", timeHistogram)

		cpuSample := metrics.NewExpDecaySample(maxDNSRequestsPerMin, 0.015)
		cpuHistogram := metrics.NewHistogram(cpuSample)
		metrics.Register("CPU Usage", cpuHistogram)

		memSample := metrics.NewExpDecaySample(maxDNSRequestsPerMin, 0.015)
		memHistogram := metrics.NewHistogram(memSample)
		metrics.Register("Mem Usage", memHistogram)

		var resultSummary map[int]DNSResult
		for i := 0; i < maxDNSRequestsPerMin; i++ {
			go MakeDNSRequestUntilSuccessful(picker, server, flowSignal, result, wg)
			mem := sigar.ProcMem{}
			if err := mem.Get(dnsServerPid); err == nil {
				memHistogram.Update(int64(mem.Resident))
				cpuFloat := getProcessCPU(dnsServerPid)
				cpuInt := cpuFloat * (1000 * 1000)
				cpuHistogram.Update(int64(cpuInt))
			}
		}
		<-finishedDNSRequestsSignal
		endTime := time.Now()

		resultSummary = buildResultSummarySync(result)

		for _, summary := range resultSummary {
			timeHistogram.Update(int64(summary.EndTime.Sub(summary.StartTime)))
		}

		return timeHistogram, memHistogram, cpuHistogram, endTime.Sub(startTime)
	}

	CheckDNSPerformanceResults := func(timeHistogram, memHistogram, cpuHistogram metrics.Histogram, duration time.Duration, medianResponseBenchmark float64) {
		medTime := timeHistogram.Percentile(0.5) / (1000 * 1000)
		maxTime := timeHistogram.Max() / (1000 * 1000)
		printStatsForHistogram(timeHistogram, fmt.Sprintf("DNS handling latency for %s", label), "ms", 1000*1000)

		maxMem := float64(memHistogram.Max()) / (1024 * 1024)
		printStatsForHistogram(memHistogram, fmt.Sprintf("DNS server mem usage for %s", label), "MB", 1024*1024)

		maxCPU := float64(cpuHistogram.Max()) / (1000 * 1000)
		printStatsForHistogram(cpuHistogram, fmt.Sprintf("DNS server CPU usage for %s", label), "%", 1000*1000)

		testFailures := []error{}
		if medTime > medianResponseBenchmark {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Median DNS response time of %.3fms was greater than %.3fms benchmark", medTime, medianResponseBenchmark)))
		}
		if maxTime > 7540 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max DNS response time of %d.000ms was greater than 7540ms benchmark", maxTime)))
		}
		if maxCPU > 5 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max DNS server CPU usage of %.2f%% was greater than 5%% ceiling", maxCPU)))
		}
		if maxMem > 15 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max DNS server memory usage of %.2fMB was greater than 15MB ceiling", maxMem)))
		}
		if duration > time.Minute {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("DNS server took %s to serve %d requests, which exceeds 1 minute benchmark", duration.String(), maxDNSRequestsPerMin)))
		}

		Expect(testFailures).To(BeEmpty())
	}

	Describe("using zones from file", func() {
		BeforeEach(func() {
			var err error
			picker, err = zp.NewZoneFilePickerFromFile("/tmp/zones.json")
			Expect(err).ToNot(HaveOccurred())
			label = "prod-like zones"
		})

		It("handles DNS responses quickly for prod like zones", func() {
			time1, mem, cpu, duration := TestDNSPerformance(dnsServerAddress)
			time2, _, _, _ := TestDNSPerformance("8.8.8.8:53")

			CheckDNSPerformanceResults(time1, mem, cpu, duration, math.MaxFloat64)
			Expect(math.Abs(time1.Percentile(0.5)-time2.Percentile(0.5))).To(BeNumerically("<", time.Millisecond),
				"expected our server to add at most 1ms to the median response time")
		})
	})

	Describe("using upcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("upcheck.bosh-dns.")
			label = "upcheck zone"
		})

		It("handles DNS responses quickly for upcheck zone", func() {
			time, mem, cpu, duration := TestDNSPerformance(dnsServerAddress)
			CheckDNSPerformanceResults(time, mem, cpu, duration, 1.5)
		})
	})

	Describe("using google zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("google.com.")
			label = "google.com zone"
		})

		It("handles DNS responses quickly for google zone", func() {
			time1, mem, cpu, duration := TestDNSPerformance(dnsServerAddress)
			time2, _, _, _ := TestDNSPerformance("8.8.8.8:53")

			CheckDNSPerformanceResults(time1, mem, cpu, duration, math.MaxFloat64)
			Expect(math.Abs(time1.Percentile(0.5)-time2.Percentile(0.5))).To(BeNumerically("<", time.Millisecond),
				"expected our server to add at most 1ms to the median response time")
		})
	})

	Describe("using local bosh dns records", func() {
		BeforeEach(func() {
			cmd := exec.Command(boshBinaryPath, []string{"scp", "dns:/var/vcap/instance/dns/records.json", "records.json"}...)
			err := cmd.Run()
			if err != nil {
				panic(fmt.Sprintf("Failed to bosh scp: %s", err.Error()))
			}

			recordsJsonBytes, err := ioutil.ReadFile("records.json")
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
			time, mem, cpu, duration := TestDNSPerformance(dnsServerAddress)
			CheckDNSPerformanceResults(time, mem, cpu, duration, 1.5)
		})
	})
})

func printStatsForHistogram(hist metrics.Histogram, label string, unit string, scalingDivisor float64) {
	fmt.Printf("\n~~~~~~~~~~~~~~~%s~~~~~~~~~~~~~~~\n", label)
	printStatNamed("Std Deviation", hist.StdDev()/scalingDivisor, unit)
	printStatNamed("Median", hist.Percentile(0.5)/scalingDivisor, unit)
	printStatNamed("Mean", hist.Mean()/scalingDivisor, unit)
	printStatNamed("Max", float64(hist.Max())/scalingDivisor, unit)
	printStatNamed("Min", float64(hist.Min())/scalingDivisor, unit)
	printStatNamed("90th Percentile", hist.Percentile(0.9)/scalingDivisor, unit)
	printStatNamed("95th Percentile", hist.Percentile(0.95)/scalingDivisor, unit)
	printStatNamed("99th Percentile", hist.Percentile(0.99)/scalingDivisor, unit)
}

func printStatNamed(label string, value float64, unit string) {
	fmt.Printf("%s: %3.3f%s\n", label, value, unit)
}

func getProcessCPU(pid int) float64 {
	cmd := exec.Command("ps", []string{"-p", strconv.Itoa(pid), "-o", "%cpu"}...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(string(output))
	}

	percentString := strings.TrimSpace(strings.Split(string(output), "\n")[1])
	percent, err := strconv.ParseFloat(percentString, 64)
	Expect(err).ToNot(HaveOccurred())

	return percent
}

func setupWaitGroupWithSignaler(maxDNSRequests int) (*sync.WaitGroup, chan struct{}) {
	wg := &sync.WaitGroup{}
	wg.Add(maxDNSRequests)
	finishedDNSRequests := make(chan struct{})

	go func() {
		wg.Wait()
		close(finishedDNSRequests)
	}()

	return wg, finishedDNSRequests
}

type DNSResult struct {
	Id        int
	RCode     int
	StartTime time.Time
	EndTime   time.Time
}

func createFlowSignal(goRoutineSize int) chan bool {
	flow := make(chan bool, goRoutineSize)
	for i := 0; i < goRoutineSize; i++ {
		flow <- true
	}

	return flow
}

func MakeDNSRequestUntilSuccessful(picker zp.ZonePicker, server string, flow chan bool, result chan DNSResult, wg *sync.WaitGroup) {
	defer func() {
		flow <- true
		wg.Done()
	}()

	<-flow
	zone := picker.NextZone()
	c := new(dns.Client)
	m := new(dns.Msg)

	m.SetQuestion(dns.Fqdn(zone), dns.TypeA)
	result <- DNSResult{Id: int(m.Id), StartTime: time.Now()}

	r := makeRequest(c, m, server)

	result <- DNSResult{Id: int(m.Id), RCode: r.Rcode, EndTime: time.Now()}
}

func makeRequest(c *dns.Client, m *dns.Msg, server string) *dns.Msg {
	r, _, err := c.Exchange(m, server)

	if err != nil {
		return makeRequest(c, m, server)
	}

	return r
}

func buildResultSummarySync(result chan DNSResult) map[int]DNSResult {
	resultSummary := make(map[int]DNSResult)
	close(result)

	for r := range result {
		if _, found := resultSummary[r.Id]; found {
			dnsResult := resultSummary[r.Id]
			dnsResult.EndTime = r.EndTime
			dnsResult.RCode = r.RCode
			resultSummary[r.Id] = dnsResult
		} else {
			resultSummary[r.Id] = r
		}
	}

	return resultSummary
}

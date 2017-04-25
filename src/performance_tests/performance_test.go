package performance_test

import (
	"github.com/cloudfoundry/gosigar"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sort"
	"sync"
	"time"

	zp "github.com/cloudfoundry/dns-release/src/performance_tests/zone_pickers"

	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

type PerformanceTestInfo struct {
	MedianRequestTime         time.Duration
	ErrorCount                int
	FailedRequestRCodesCounts map[int]int
	MaxRuntime                time.Duration
}

var _ = Describe("Performance", func() {
	var maxDnsRequestsPerMin int
	var info PerformanceTestInfo
	var flowSignal chan bool
	var wg *sync.WaitGroup
	var finishedDnsRequestsSignal chan struct{}
	var result chan DnsResult
	var picker zp.ZonePicker
	var label string

	var dnsServerPid int

	BeforeEach(func() {
		var found bool
		dnsServerPid, found = GetPidFor("dns")
		Expect(found).To(BeTrue())

		maxDnsRequestsPerMin = 420
		info = PerformanceTestInfo{}

		flowSignal = createFlowSignal(10)
		wg, finishedDnsRequestsSignal = setupWaitGroupWithSignaler(maxDnsRequestsPerMin)
		result = make(chan DnsResult, maxDnsRequestsPerMin*2)
	})

	TestDNSPerformance := func() {
		startTime := time.Now()
		memUsageValues := []float64{}
		CPUUsageValues := []float64{}

		var resultSummary map[int]DnsResult
		for i := 0; i < maxDnsRequestsPerMin; i++ {
			go MakeDnsRequestUntilSuccessful(picker, flowSignal, result, wg)
			mem := sigar.ProcMem{}
			if err := mem.Get(dnsServerPid); err == nil {
				memUsageValues = append(memUsageValues, float64(mem.Resident)/1024/1024)
				CPUUsageValues = append(CPUUsageValues, getProcessCPU(dnsServerPid))
			}
		}
		<-finishedDnsRequestsSignal
		endTime := time.Now()

		resultSummary = buildResultSummarySync(result)

		resultTimes := []int{}
		for _, summary := range resultSummary {
			resultTimes = append(resultTimes, int(summary.EndTime.Sub(summary.StartTime)))
		}

		sort.Ints([]int(resultTimes))
		median := (time.Duration(resultTimes[209]) + time.Duration(resultTimes[210])) / 2
		max := time.Duration(resultTimes[len(resultTimes)-1])

		sort.Float64s(memUsageValues)
		maxMem := memUsageValues[len(memUsageValues)-1]

		sort.Float64s(CPUUsageValues)
		maxCPU := CPUUsageValues[len(CPUUsageValues)-1]

		Expect(endTime).Should(BeTemporally("<", startTime.Add(1*time.Minute)))

		fmt.Printf("Median DNS response time for %s: %s\n", label, median.String())
		Expect(median).To(BeNumerically("<", 797*time.Microsecond))

		fmt.Printf("Max DNS response time for %s: %s\n", label, max.String())
		Expect(max).To(BeNumerically("<", 7540190*time.Microsecond))

		fmt.Printf("Max DNS server memory usage for %s: %f Mb\n", label, maxMem)
		Expect(maxMem).To(BeNumerically("<", 15))

		fmt.Printf("Max DNS server CPU usage for %s: %f %%\n", label, maxCPU)
		Expect(maxCPU).To(BeNumerically("<", 5))
	}

	Describe("using zones from file", func() {
		BeforeEach(func() {
			var err error
			picker, err = zp.NewZoneFilePickerFromFile("/tmp/zones.json")
			Expect(err).ToNot(HaveOccurred())
			label = "prod-like zones"
		})

		It("handles DNS responses quickly for prod like zones", func() {
			TestDNSPerformance()
		})
	})

	Describe("using healthcheck zone", func() {
		BeforeEach(func() {
			picker = zp.NewStaticZonePicker("healthcheck.bosh-dns.")
			label = "healthcheck zone"
		})

		It("handles DNS responses quickly for healthcheck zone", func() {
			TestDNSPerformance()
		})
	})

	Describe("using local bosh dns records", func() {
		BeforeEach(func() {
			cmd := exec.Command(boshBinaryPath, []string{"scp", "dns:/var/vcap/instance/dns/records.json", "records.json"}...)
			err := cmd.Run()
			if err != nil {
				panic(fmt.Sprintf("Failed to bosh scp: %s\nOutput: %s", err.Error()))
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
			TestDNSPerformance()
		})
	})
})

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

func setupWaitGroupWithSignaler(maxDnsRequests int) (*sync.WaitGroup, chan struct{}) {
	wg := &sync.WaitGroup{}
	wg.Add(maxDnsRequests)
	finishedDnsRequests := make(chan struct{})

	go func() {
		wg.Wait()
		close(finishedDnsRequests)
	}()

	return wg, finishedDnsRequests
}

type DnsResult struct {
	Id        int
	RCode     int
	StartTime time.Time
	EndTime   time.Time
}

func createFlowSignal(goRoutineSize int) chan bool {
	flow := make(chan bool, goRoutineSize)
	for i := 0; i < 10; i++ {
		flow <- true
	}

	return flow
}

func MakeDnsRequestUntilSuccessful(picker zp.ZonePicker, flow chan bool, result chan DnsResult, wg *sync.WaitGroup) {
	defer func() {
		flow <- true
		wg.Done()
	}()

	<-flow
	zone := picker.NextZone()
	c := new(dns.Client)
	m := new(dns.Msg)

	m.SetQuestion(dns.Fqdn(zone), dns.TypeA)
	result <- DnsResult{Id: int(m.Id), StartTime: time.Now()}

	r := makeRequest(c, m)

	result <- DnsResult{Id: int(m.Id), RCode: r.Rcode, EndTime: time.Now()}
}

func makeRequest(c *dns.Client, m *dns.Msg) *dns.Msg {
	r, _, err := c.Exchange(m, "10.245.0.34:53")

	if err != nil {
		return makeRequest(c, m)
	}

	return r
}

func buildResultSummarySync(result chan DnsResult) map[int]DnsResult {
	resultSummary := make(map[int]DnsResult)
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

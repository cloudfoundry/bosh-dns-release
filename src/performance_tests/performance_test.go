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

	var dnsServerPid int

	testDnsPerformance := func(picker zp.ZonePicker) {
		Context("420 req / min", func() {
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

			Measure("should handle with less than 0.797ms median", func(b Benchmarker) {
				concreteSigar := sigar.ConcreteSigar{}
				cpuChannel, stopCpuChannel := concreteSigar.CollectCpuStats(100 * time.Millisecond)

				b.Time("420 DNS queries", func() {
					for i := 0; i < maxDnsRequestsPerMin; i++ {
						go MakeDnsRequestUntilSuccessful(picker, flowSignal, result, wg)
						mem := sigar.ProcMem{}
						if err := mem.Get(dnsServerPid); err == nil {
							b.RecordValue("DNS Server Memory Usage (in Mb)", float64(mem.Resident/1024/1024))
						}
					}
					<-finishedDnsRequestsSignal
					close(stopCpuChannel)
				})

				cpuResult := <-cpuChannel
				b.RecordValue("Total CPU Usage (%)", (float64(cpuResult.User+cpuResult.Sys)/float64(cpuResult.Total()))*100)
			}, 5)

			It("handles DNS responses quickly", func() {
				startTime := time.Now()
				var resultSummary map[int]*DnsResult
				for i := 0; i < maxDnsRequestsPerMin; i++ {
					go MakeDnsRequestUntilSuccessful(picker, flowSignal, result, wg)
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

				Expect(endTime).Should(BeTemporally("<", startTime.Add(1*time.Minute)))
				Expect(median).To(BeNumerically("<", 797*time.Microsecond))
			})
		})
	}

	Describe("using zones from file", func() {
		picker, _ := zp.NewJsonFileZonePicker("/tmp/zones.json")
		testDnsPerformance(picker)
	})

	Describe("using healthcheck zone", func() {
		picker := zp.NewStaticZonePicker("healthcheck.bosh-dns.")
		testDnsPerformance(picker)
	})
})

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

func buildResultSummarySync(result chan DnsResult) map[int]*DnsResult {
	resultSummary := make(map[int]*DnsResult)
	close(result)

	for r := range result {
		if _, found := resultSummary[r.Id]; found {
			dnsResult := resultSummary[r.Id]
			dnsResult.EndTime = r.EndTime
			dnsResult.RCode = r.RCode
		} else {
			resultSummary[r.Id] = &r
		}
	}

	return resultSummary
}

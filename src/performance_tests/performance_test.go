package performance_test

import (
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"time"
	"sort"
)

type ZonePicker interface {
	NextZone() string
}

type DnsResult struct {
	Id        int
	RCode     int
	StartTime time.Time
	EndTime   time.Time
	Error error
}

type GooglePicker struct {}

func (GooglePicker) NextZone() string {
	return "google.com."
}

func initializeFlow(flow chan bool) {
	for i := 0; i < 10; i++ {
		flow <- true
	}
}

func MakeDnsRequest(picker ZonePicker, flow chan bool, result chan DnsResult, wg *sync.WaitGroup) error {
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
	r, _, err := c.Exchange(m, "169.254.0.2:53")
	if err != nil {
		result <- DnsResult{Id: int(m.Id), Error: err, EndTime: time.Now()}
		return err
	}

	result <- DnsResult{Id: int(m.Id), RCode: r.Rcode, EndTime: time.Now()}
	return nil
}

func buildResultSummary(result chan DnsResult, finishedDnsRequests chan struct{}) map[int]*DnsResult {
	resultSummary := make(map[int]*DnsResult)
	for {
		select {
		case r := <-result:
			if _, found := resultSummary[r.Id]; found {
				dnsResult := resultSummary[r.Id]
				dnsResult.EndTime = r.EndTime
				dnsResult.RCode = r.RCode
			} else {
				resultSummary[r.Id] = &r
			}
		case <-finishedDnsRequests:
			return resultSummary
		}
	}
}

var _ = Describe("Performance", func() {
	It("should handle 420 req / min with less than 0.797ms median", func() {
		result := make(chan DnsResult)
		finishedDnsRequests := make(chan struct{})
		wg := &sync.WaitGroup{}
		maxDnsRequests := 420
		goRoutineSize := 10
		flow := make(chan bool, goRoutineSize)

		initializeFlow(flow)

		startTime := time.Now()
		wg.Add(maxDnsRequests)
		for i := 0; i < maxDnsRequests; i++ {
			go MakeDnsRequest(GooglePicker{}, flow, result, wg)
		}

		go func() {
			wg.Wait()
			close(finishedDnsRequests)
		}()

		resultSummary := buildResultSummary(result, finishedDnsRequests)
		endTime := time.Now()
		resultTimes := []int{}

		for _, summary := range resultSummary {
			Expect(summary.Error).ToNot(HaveOccurred())
			Expect(summary.RCode).To(Equal(dns.RcodeSuccess))
			resultTimes = append(resultTimes, int(summary.EndTime.Sub(summary.StartTime)))
		}

		sort.Ints([]int(resultTimes))
		median := (time.Duration(resultTimes[209]) + time.Duration(resultTimes[210])) / 2

		Expect(endTime).To(BeTemporally("<", startTime.Add(1*time.Minute)))
		Expect(median).To(BeNumerically("<", 797*time.Microsecond))
	})
})


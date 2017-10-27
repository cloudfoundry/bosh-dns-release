package performance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	sigar "github.com/cloudfoundry/gosigar"
	. "github.com/onsi/gomega"

	metrics "github.com/rcrowley/go-metrics"
)

type Result struct {
	status     int
	value      interface{}
	metricName string
	time       int64
}

type TimeThresholds struct {
	Max, Med, Pct90, Pct95 time.Duration
}

type VitalsThresholds struct {
	CPUPct99 float64
	MemPct99 float64
}

func TimeThresholdsFromBenchmark(benchmark metrics.Histogram, allowance float64) TimeThresholds {
	return TimeThresholds{
		Max:   7540 * time.Millisecond,
		Med:   time.Duration(float64(benchmark.Percentile(0.5)) * allowance),
		Pct90: time.Duration(float64(benchmark.Percentile(0.9)) * allowance),
		Pct95: time.Duration(float64(benchmark.Percentile(0.95)) * allowance),
	}
}

type PerformanceTest struct {
	Application string
	Context     string

	Workers           int
	RequestsPerSecond int

	ServerPID int

	TimeThresholds   TimeThresholds
	VitalsThresholds VitalsThresholds

	SuccessStatus int

	WorkerFunc func(chan<- Result)

	shutdown chan struct{}
}

func (p PerformanceTest) Setup() *PerformanceTest {
	p.shutdown = make(chan struct{})
	return &p
}

func validateDatadogEnvironment() (string, string) {
	environment := os.Getenv("DATADOG_ENVIRONMENT_TAG")
	if environment == "" {
		panic("Need to set DATADOG_ENVIRONMENT_TAG to prevent creating bogus data buckets")
	}

	gitSHA := os.Getenv("GIT_SHA")
	if gitSHA == "" {
		panic("Need to set GIT_SHA to correlate performance metric to release")
	}

	return environment, gitSHA
}

func makeDataDogRequest(endpoint string, content interface{}) error {
	apiKey := os.Getenv("DATADOG_API_KEY")
	if apiKey == "" {
		panic("DATADOG_API_KEY is missing")
	}

	uri := fmt.Sprintf("https://app.datadoghq.com/api/v1/%s?api_key=%s", endpoint, apiKey)

	jsonContents, err := json.Marshal(content)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(jsonContents)
	_, err = http.Post(uri, "application/json", buf)
	return err
}

func (p *PerformanceTest) postDatadogEvent(title, text string) error {
	environment, gitSHA := validateDatadogEnvironment()

	event := struct {
		Title     string   `json:"title"`
		Text      string   `json:"text"`
		Priority  string   `json:"priority"`
		Tags      []string `json:"tags"`
		AlertType string   `json:"alert_type"`
	}{
		Title:     title,
		Text:      text,
		Priority:  "normal",
		AlertType: "info",
		Tags: []string{
			fmt.Sprintf("environment:%s", environment),
			fmt.Sprintf("application:%s", p.Application),
			fmt.Sprintf("context:%s", p.Context),
			fmt.Sprintf("sha:%s", gitSHA),
		},
	}

	return makeDataDogRequest("events", event)
}

type Series struct {
	Metric string          `json:"metric"`
	Points [][]interface{} `json:"points"`
	Type   string          `json:"type"`
	Tags   []string        `json:"tags"`
}

type Items map[string][]Series

func (p *PerformanceTest) postDatadog(r ...Result) error {
	environment, gitSHA := validateDatadogEnvironment()

	metrics := []Series{}
	for _, v := range r {
		metrics = append(metrics, Series{
			Metric: v.metricName,
			Points: [][]interface{}{{v.time, v.value}},
			Type:   "gauge",
			Tags: []string{
				fmt.Sprintf("environment:%s", environment),
				fmt.Sprintf("application:%s", p.Application),
				fmt.Sprintf("context:%s", p.Context),
				fmt.Sprintf("sha:%s", gitSHA),
			},
		})
	}

	metric := Items{"series": metrics}

	return makeDataDogRequest("series", metric)
}

func (p *PerformanceTest) MakeParallelRequests(duration time.Duration) []Result {
	wg := &sync.WaitGroup{}
	resultChan := make(chan Result, p.RequestsPerSecond)

	workerFunc := func(wg *sync.WaitGroup, maxRequestsPerSecond float64) {
		buffer := make(chan struct{}, 2*int(math.Ceil(maxRequestsPerSecond)))
		ticker := time.NewTicker(time.Duration(float64(time.Second) / maxRequestsPerSecond))
		buffer <- struct{}{}

		defer func() {
			wg.Done()
			ticker.Stop()
		}()

		go func() {
			for {
				select {
				case <-p.shutdown:
					return
				case <-ticker.C:
					buffer <- struct{}{}
				}
			}
		}()

		for {
			select {
			case <-p.shutdown:
				return
			case <-buffer:
				p.WorkerFunc(resultChan)
			}
		}
	}

	doneChan := make(chan struct{})
	results := []Result{}

	dataDogDoneChan := make(chan struct{})
	dataDogResults := make(chan []Result, p.RequestsPerSecond*int(duration/time.Second))

	requestPerSecondTicker := time.NewTicker(time.Duration(1 * time.Second))
	go func() {
		successCount := 0
		totalRequestsPerSecond := 0
		for {
			select {
			case result, ok := <-resultChan:
				if ok == false {
					close(doneChan)
					close(dataDogResults)
					return
				}
				if result.status == p.SuccessStatus {
					successCount += 1
				}
				totalRequestsPerSecond++
				dataDogResults <- []Result{result}
				results = append(results, result)
			case <-requestPerSecondTicker.C:
				vals := []Result{
					{
						status:     0,
						value:      successCount,
						metricName: "succesful_requests_per_second",
						time:       time.Now().Unix(),
					},
					{
						status:     0,
						value:      totalRequestsPerSecond,
						metricName: "total_requests_per_second",
						time:       time.Now().Unix(),
					}}
				dataDogResults <- vals
				successCount = 0
				totalRequestsPerSecond = 0
			}
		}
	}()

	go func() {
		for results := range dataDogResults {
			p.postDatadog(results...)
		}
		close(dataDogDoneChan)
	}()

	wg.Add(p.Workers)
	for i := 0; i < p.Workers; i++ {
		go workerFunc(wg, float64(p.RequestsPerSecond)/float64(p.Workers))
	}

	time.Sleep(duration)
	close(p.shutdown)

	wg.Wait()
	close(resultChan)
	<-doneChan
	<-dataDogDoneChan

	return results
}

func (p *PerformanceTest) TestPerformance(durationInSeconds int, label string) {
	p.postDatadogEvent("Starting performance test", "")
	duration := time.Duration(durationInSeconds) * time.Second
	resourcesInterval := time.Second / 2

	cpuSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
	cpuHistogram := metrics.NewHistogram(cpuSample)
	metrics.Register("CPU Usage", cpuHistogram)

	memSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
	memHistogram := metrics.NewHistogram(memSample)
	metrics.Register("Mem Usage", memHistogram)

	done := make(chan struct{})
	go p.measureResourceUtilization(resourcesInterval, cpuHistogram, memHistogram, done)

	results := p.MakeParallelRequests(duration)
	<-done

	timeHistogram := generateTimeHistogram(results)

	successCount := 0
	for _, hr := range results {
		if hr.status == p.SuccessStatus {
			successCount++
		}
	}

	successPercentage := float64(successCount) / float64(len(results))
	fmt.Printf("success percentage is %.02f%%\n", successPercentage*100)
	fmt.Printf("requests per second is %d reqs/s\n", successCount/durationInSeconds)

	medTime := time.Duration(timeHistogram.Percentile(0.5))
	pct90Time := time.Duration(timeHistogram.Percentile(0.9))
	pct95Time := time.Duration(timeHistogram.Percentile(0.95))
	printStatsForHistogram(timeHistogram, fmt.Sprintf("Handling latency for %s", label), "ms", 1000*1000)

	mem99Pct := float64(memHistogram.Percentile(0.99)) / (1024 * 1024)
	printStatsForHistogram(memHistogram, fmt.Sprintf("Server mem usage for %s", label), "MB", 1024*1024)

	cpu99Pct := float64(cpuHistogram.Percentile(0.99)) / (1000 * 1000)
	printStatsForHistogram(cpuHistogram, fmt.Sprintf("Server CPU usage for %s", label), "%", 1000*1000)

	testFailures := []error{}
	marginOfError := 0.05
	requestsPerSecondThreshold := int((1.0 - marginOfError) * float64(p.RequestsPerSecond))
	if (successCount / durationInSeconds) < requestsPerSecondThreshold {
		testFailures = append(testFailures,
			fmt.Errorf("Handled requests %d per second was lower than %d benchmark", successCount/durationInSeconds, requestsPerSecondThreshold))
	}
	if successCount < len(results) {
		testFailures = append(testFailures,
			fmt.Errorf("Success ratio %d/%d, giving percentage %.3f%%, is too low", successCount, len(results), 100*successPercentage))
	}

	if medTime > p.TimeThresholds.Med {
		testFailures = append(testFailures,
			fmt.Errorf("Median response time of %.3fms was greater than %.3fms benchmark",
				float64(medTime)/float64(time.Millisecond),
				float64(p.TimeThresholds.Med)/float64(time.Millisecond)))
	}
	if pct90Time > p.TimeThresholds.Pct90 {
		testFailures = append(testFailures,
			fmt.Errorf("90th percentile response time of %.3fms was greater than %.3fms benchmark",
				float64(pct90Time)/float64(time.Millisecond),
				float64(p.TimeThresholds.Pct90)/float64(time.Millisecond)))
	}
	if pct95Time > p.TimeThresholds.Pct95 {
		testFailures = append(testFailures,
			fmt.Errorf("95th percentile response time of %.3fms was greater than %.3fms benchmark",
				float64(pct95Time)/float64(time.Millisecond),
				float64(p.TimeThresholds.Pct95)/float64(time.Millisecond)))
	}

	if cpu99Pct > p.VitalsThresholds.CPUPct99 {
		testFailures = append(testFailures,
			fmt.Errorf("99th percentile server CPU usage of %.2f%% was greater than %.2f%% ceiling", cpu99Pct, p.VitalsThresholds.CPUPct99))
	}

	if mem99Pct > p.VitalsThresholds.MemPct99 {
		testFailures = append(testFailures,
			fmt.Errorf("99th percentile server memory usage of %.2fMB was greater than %.2fMB ceiling", mem99Pct, p.VitalsThresholds.MemPct99))
	}

	p.postDatadogEvent("Finishing performance test", "")
	Expect(testFailures).To(BeEmpty())
}

func (p *PerformanceTest) getProcessCPU() float64 {
	cmd := exec.Command("ps", []string{"-p", strconv.Itoa(p.ServerPID), "-o", "%cpu"}...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(string(output) + err.Error())
	}

	percentString := strings.TrimSpace(strings.Split(string(output), "\n")[1])
	percent, err := strconv.ParseFloat(percentString, 64)
	Expect(err).ToNot(HaveOccurred())

	return percent
}

func (p *PerformanceTest) measureResourceUtilization(resourcesInterval time.Duration, cpuHistogram, memHistogram metrics.Histogram, done chan<- struct{}) {
	ticker := time.NewTicker(resourcesInterval)
	defer func() {
		ticker.Stop()
		close(done)
	}()

	for {
		select {
		case <-p.shutdown:
			return
		case <-ticker.C:
			mem := sigar.ProcMem{}
			if err := mem.Get(p.ServerPID); err == nil {
				memHistogram.Update(int64(mem.Resident))
				cpuFloat := p.getProcessCPU()
				cpuInt := cpuFloat * (1000 * 1000)
				cpuHistogram.Update(int64(cpuInt))

				go func(t int64) {
					p.postDatadog(
						Result{
							metricName: "memory",
							value:      mem.Resident,
							time:       t,
						},
						Result{
							metricName: "cpu",
							value:      cpuInt,
							time:       t,
						},
					)
				}(time.Now().Unix())
			}
		}
	}
}

func generateTimeHistogram(results []Result) metrics.Histogram {
	timeSample := metrics.NewExpDecaySample(len(results), 0.015)
	timeHistogram := metrics.NewHistogram(timeSample)
	for _, result := range results {
		timeHistogram.Update(int64(result.value.(time.Duration)))
	}
	return timeHistogram
}

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

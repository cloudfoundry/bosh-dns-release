package performance_test

import (
	"fmt"
	"math"
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
	status       int
	responseTime time.Duration
}

type PerformanceTest struct {
	Workers           int
	RequestsPerSecond int

	MaxTimeThreshold time.Duration
	MedTimeThreshold time.Duration

	ServerPID int

	CPUThresholdMax   float64
	CPUThresholdPct99 float64
	MemThresholdMax   float64

	SuccessStatus int

	WorkerFunc func(chan<- Result)

	shutdown chan struct{}
}

func (p PerformanceTest) Setup() *PerformanceTest {
	p.shutdown = make(chan struct{})
	return &p
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
	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
		close(doneChan)
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

	return results
}

func (p *PerformanceTest) TestPerformance(durationInSeconds int, label string) {
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
	maxTime := time.Duration(timeHistogram.Max())
	printStatsForHistogram(timeHistogram, fmt.Sprintf("Handling latency for %s", label), "ms", 1000*1000)

	memMax := float64(memHistogram.Max()) / (1024 * 1024)
	printStatsForHistogram(memHistogram, fmt.Sprintf("Server mem usage for %s", label), "MB", 1024*1024)

	cpuMax := float64(cpuHistogram.Max()) / (1000 * 1000)
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
			fmt.Errorf("Success count %d is less than total count %d. Success percentage %.1f%% is too low", successCount, len(results), 100*successPercentage))
	}
	if medTime > p.MedTimeThreshold {
		testFailures = append(testFailures,
			fmt.Errorf("Median response time of %.3fms was greater than %.3fms benchmark",
				float64(medTime/time.Millisecond),
				float64(p.MedTimeThreshold/time.Millisecond)))
	}
	if maxTime > p.MaxTimeThreshold {
		testFailures = append(testFailures,
			fmt.Errorf("Max response time of %.3fms was greater than %.3fms benchmark",
				float64(maxTime/time.Millisecond),
				float64(p.MaxTimeThreshold/time.Millisecond)))
	}
	if cpuMax > p.CPUThresholdMax {
		testFailures = append(testFailures,
			fmt.Errorf("Max server CPU usage of %.2f%% was greater than %.2f%% ceiling", cpuMax, p.CPUThresholdMax))
	}

	if cpu99Pct > p.CPUThresholdPct99 {
		testFailures = append(testFailures,
			fmt.Errorf("99th percentile server CPU usage of %.2f%% was greater than %.2f%% ceiling", cpu99Pct, p.CPUThresholdPct99))
	}

	if memMax > p.MemThresholdMax {
		testFailures = append(testFailures,
			fmt.Errorf("Max server memory usage of %.2fMB was greater than %.2fMB ceiling", memMax, p.MemThresholdMax))
	}

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
			}
		}
	}
}

func generateTimeHistogram(results []Result) metrics.Histogram {
	timeSample := metrics.NewExpDecaySample(len(results), 0.015)
	timeHistogram := metrics.NewHistogram(timeSample)
	for _, result := range results {
		timeHistogram.Update(int64(result.responseTime))
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

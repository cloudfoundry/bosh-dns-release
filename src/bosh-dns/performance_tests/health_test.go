package performance_test

import (
	"bosh-dns/healthcheck/healthclient"
	"errors"
	"io/ioutil"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rcrowley/go-metrics"

	"fmt"

	"crypto/tls"
	"crypto/x509"

	"net/http"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	sigar "github.com/cloudfoundry/gosigar"
)

type HealthResult struct {
	Id           int
	responseTime time.Duration
	status       int
}

var _ = XDescribe("Health Server", func() {
	var (
		serverAddress     = "10.245.0.34:8853"
		durationInSeconds = 5
		workers           = 10
	)

	TestHealthPerformance := func(serverAddress string, requestsPerSecond int, maxExpectedMedianTimeInMs float64) {
		wg := &sync.WaitGroup{}
		httpClient := setupSecureGet()

		healthResult := make(chan HealthResult, requestsPerSecond)
		healthServerPID, found := GetPidFor("dns-health")
		Expect(found).To(BeTrue())

		shutdown := make(chan struct{})

		workerFunc := func(wg *sync.WaitGroup, maxRequestsPerSecond int, shutdown chan struct{}) {
			timer := time.NewTicker(time.Second / time.Duration(maxRequestsPerSecond))
			defer func() {
				wg.Done()
				timer.Stop()
			}()

			for {
				select {
				case <-shutdown:
					return
				case <-timer.C:
					MakeHealthEndpointRequest(httpClient, serverAddress, healthResult)
				}
			}
		}

		doneChan := make(chan struct{})
		results := []HealthResult{}
		go func() {
			for hr := range healthResult {
				results = append(results, hr)
			}
			close(doneChan)
		}()

		cpuSample := metrics.NewExpDecaySample(requestsPerSecond*durationInSeconds*2, 0.015)
		cpuHistogram := metrics.NewHistogram(cpuSample)
		metrics.Register("CPU Usage", cpuHistogram)

		memSample := metrics.NewExpDecaySample(requestsPerSecond*durationInSeconds*2, 0.015)
		memHistogram := metrics.NewHistogram(memSample)
		metrics.Register("Mem Usage", memHistogram)

		mem := sigar.ProcMem{}
		if err := mem.Get(healthServerPID); err == nil {
			fmt.Println("initial mem: ", mem.Resident)
		}

		go func() {
			timer := time.NewTicker(500 * time.Millisecond)
			defer timer.Stop()

			for {
				select {
				case <-shutdown:
					return
				case <-timer.C:
					mem := sigar.ProcMem{}
					if err := mem.Get(healthServerPID); err == nil {
						memHistogram.Update(int64(mem.Resident))
						cpuFloat := getProcessCPU(healthServerPID)
						cpuInt := cpuFloat * (1000 * 1000)
						cpuHistogram.Update(int64(cpuInt))
					}
				}
			}
		}()

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go workerFunc(wg, 2*requestsPerSecond/workers, shutdown)
		}

		time.Sleep(time.Duration(durationInSeconds) * time.Second)
		close(shutdown)
		wg.Wait()
		close(healthResult)
		<-doneChan
		timeSample := metrics.NewExpDecaySample(len(results), 0.015)
		timeHistogram := metrics.NewHistogram(timeSample)
		successCount := 0
		for _, hr := range results {
			timeHistogram.Update(int64(hr.responseTime))
			if hr.status == 200 {
				successCount++
			}
		}

		medTimeInMs := timeHistogram.Percentile(0.5) / float64(time.Millisecond)
		maxTimeInMs := timeHistogram.Max() / int64(time.Millisecond)
		successPercentage := float64(successCount) / float64(len(results))
		fmt.Printf("success percentage is %f\n", successPercentage)
		printStatsForHistogram(timeHistogram, "Health server latency", "ms", float64(time.Millisecond))

		maxMem := float64(memHistogram.Max()) / (1024 * 1024)
		printStatsForHistogram(memHistogram, fmt.Sprintf("Health server mem usage"), "MB", 1024*1024)

		maxCPU := float64(cpuHistogram.Max()) / (1000 * 1000)
		printStatsForHistogram(cpuHistogram, fmt.Sprintf("Health server CPU usage"), "%", 1000*1000)

		testFailures := []error{}
		if (successCount / durationInSeconds) < requestsPerSecond {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Handled Health requests %d per second was lower than %d benchmark", (successCount/durationInSeconds), requestsPerSecond)))
		}
		if successPercentage < 1 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Health success percentage of %.1f%% is too low", 100*successPercentage)))
		}
		if medTimeInMs > maxExpectedMedianTimeInMs {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Median Health response time of %.3fms was greater than %.3fms benchmark", medTimeInMs, maxExpectedMedianTimeInMs)))
		}
		if maxTimeInMs > 7540 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max Health response time of %d.000ms was greater than 7540ms benchmark", maxTimeInMs)))
		}
		if maxCPU > 50 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max Health server CPU usage of %.2f%% was greater than 50%% ceiling", maxCPU)))
		}
		if maxMem > 15 {
			testFailures = append(testFailures, errors.New(fmt.Sprintf("Max Health server memory usage of %.2fMB was greater than 15MB ceiling", maxMem)))
		}

		Expect(testFailures).To(BeEmpty())
	}

	Describe("health server performance", func() {
		It("handles requests quickly", func() {
			TestHealthPerformance(serverAddress, 400, 10)
		})
	})
})

func MakeHealthEndpointRequest(client httpclient.HTTPClient, serverAddress string, hr chan HealthResult) {
	startTime := time.Now()
	resp, err := secureGetHealthEndpoint(client, serverAddress)
	responseTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("Error hitting health endpoint: %s\n", err.Error())
		hr <- HealthResult{status: http.StatusRequestTimeout, responseTime: responseTime}
	} else {
		hr <- HealthResult{status: resp.StatusCode, responseTime: responseTime}
	}
}

func setupSecureGet() httpclient.HTTPClient {
	cmdRunner := system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/certificate",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	clientCertificate := stdOut

	stdOut, stdErr, exitStatus, err = cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/private_key",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	clientPrivateKey := stdOut

	stdOut, stdErr, exitStatus, err = cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/ca",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	caCert := stdOut

	cert, err := tls.X509KeyPair([]byte(clientCertificate), []byte(clientPrivateKey))
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard, ioutil.Discard)

	return healthclient.NewHealthClient([]byte(caCert), cert, logger)
}

func secureGetHealthEndpoint(client httpclient.HTTPClient, serverAddress string) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s/health", serverAddress))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

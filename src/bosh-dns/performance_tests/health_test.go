package performance_test

import (
	"bosh-dns/healthcheck/healthclient"
	"io/ioutil"
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
)

var _ = Describe("Health Server", func() {
	var (
		serverAddress     = "127.0.0.1:8853"
		durationInSeconds = 60
		workers           = 10
		requestsPerSecond = 400
	)

	TestHealthPerformance := func(serverAddress string, requestsPerSecond int, maxExpectedMedianTimeInMs float64) {
		httpClient := setupSecureGet()

		shutdown := make(chan struct{})

		duration := time.Duration(durationInSeconds) * time.Second
		resourcesInterval := time.Second / 2

		cpuSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
		cpuHistogram := metrics.NewHistogram(cpuSample)
		metrics.Register("CPU Usage", cpuHistogram)

		memSample := metrics.NewExpDecaySample(int(duration/resourcesInterval), 0.015)
		memHistogram := metrics.NewHistogram(memSample)
		metrics.Register("Mem Usage", memHistogram)

		done := make(chan struct{})
		go measureResourceUtilization(healthSession.Command.Process.Pid, resourcesInterval, cpuHistogram, memHistogram, done, shutdown)

		results := makeParallelRequests(requestsPerSecond, workers, duration, shutdown, func(resultChan chan<- Result) {
			MakeHealthEndpointRequest(httpClient, serverAddress, resultChan)
		})
		<-done

		timeHistogram := generateTimeHistogram(results)

		successCount := 0
		for _, hr := range results {
			if hr.status == 200 {
				successCount++
			}
		}

		successPercentage := float64(successCount) / float64(len(results))
		fmt.Printf("success percentage is %.02f%%\n", successPercentage*100)
		fmt.Printf("requests per second is %d reqs/s\n", successCount/durationInSeconds)

		medTime := timeHistogram.Percentile(0.5) / float64(time.Millisecond)
		maxTime := timeHistogram.Max() / int64(time.Millisecond)
		printStatsForHistogram(timeHistogram, "Health server latency", "ms", float64(time.Millisecond))

		maxMem := float64(memHistogram.Max()) / (1024 * 1024)
		printStatsForHistogram(memHistogram, fmt.Sprintf("Health server mem usage"), "MB", 1024*1024)

		maxCPU := float64(cpuHistogram.Max()) / (1000 * 1000)
		printStatsForHistogram(cpuHistogram, fmt.Sprintf("Health server CPU usage"), "%", 1000*1000)

		testFailures := []error{}
		if (successCount / durationInSeconds) < requestsPerSecond {
			testFailures = append(testFailures, fmt.Errorf("Handled Health requests %d per second was lower than %d benchmark", successCount/durationInSeconds, requestsPerSecond))
		}
		if successPercentage < 1 {
			testFailures = append(testFailures, fmt.Errorf("Health success percentage of %.1f%% is too low", 100*successPercentage))
		}
		if medTime > maxExpectedMedianTimeInMs {
			testFailures = append(testFailures, fmt.Errorf("Median Health response time of %.3fms was greater than %.3fms benchmark", medTime, maxExpectedMedianTimeInMs))
		}
		maxTimeinMsThreshold := int64(7540)
		if maxTime > maxTimeinMsThreshold {
			testFailures = append(testFailures, fmt.Errorf("Max Health response time of %d.000ms was greater than %d.000ms benchmark", maxTime, maxTimeinMsThreshold))
		}
		cpuThresholdPercentage := float64(50)
		if maxCPU > cpuThresholdPercentage {
			testFailures = append(testFailures, fmt.Errorf("Max Health server CPU usage of %.2f%% was greater than %.2f%% ceiling", maxCPU, cpuThresholdPercentage))
		}
		memThreshold := float64(15)
		if maxMem > memThreshold {
			testFailures = append(testFailures, fmt.Errorf("Max Health server memory usage of %.2fMB was greater than %.2fMB ceiling", maxMem, memThreshold))
		}

		Expect(testFailures).To(BeEmpty())
	}

	Describe("health server performance", func() {
		It("handles requests quickly", func() {
			TestHealthPerformance(serverAddress, requestsPerSecond, 10)
		})
	})
})

func MakeHealthEndpointRequest(client httpclient.HTTPClient, serverAddress string, hr chan<- Result) {
	startTime := time.Now()
	resp, err := secureGetHealthEndpoint(client, serverAddress)
	responseTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("Error hitting health endpoint: %s\n", err.Error())
		hr <- Result{status: http.StatusRequestTimeout, responseTime: responseTime}
	} else {
		hr <- Result{status: resp.StatusCode, responseTime: responseTime}
	}
}

func setupSecureGet() httpclient.HTTPClient {
	// Load client cert
	cert, err := tls.LoadX509KeyPair("../healthcheck/assets/test_certs/test_client.pem", "../healthcheck/assets/test_certs/test_client.key")
	Expect(err).NotTo(HaveOccurred())

	// Load CA cert
	caCert, err := ioutil.ReadFile("../healthcheck/assets/test_certs/test_ca.pem")
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard, ioutil.Discard)

	return healthclient.NewHealthClient(caCert, cert, logger)
}

func secureGetHealthEndpoint(client httpclient.HTTPClient, serverAddress string) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s/health", serverAddress))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

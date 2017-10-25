package performance_test

import (
	"bosh-dns/healthcheck/healthclient"
	"io/ioutil"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
		durationInSeconds = 60 * 30
		workers           = 10
		requestsPerSecond = 400
	)

	TestHealthPerformance := func(timeThresholds TimeThresholds) {
		httpClient := setupSecureGet()

		PerformanceTest{
			Application: "health",
			Context:     "health",

			Workers:           workers,
			RequestsPerSecond: requestsPerSecond,

			ServerPID: healthSessions[0].Command.Process.Pid,

			TimeThresholds: timeThresholds,
			VitalsThresholds: VitalsThresholds{
				CPUPct99: 60,
				MemPct99: 20,
			},

			SuccessStatus: http.StatusOK,

			WorkerFunc: func(resultChan chan<- Result) {
				MakeHealthEndpointRequest(httpClient, serverAddress, resultChan)
			},
		}.Setup().TestPerformance(durationInSeconds, "health")
	}

	Describe("health server performance", func() {
		It("handles requests quickly", func() {
			TestHealthPerformance(TimeThresholds{
				Med:   15 * time.Millisecond,
				Pct90: 20 * time.Millisecond,
				Pct95: 25 * time.Millisecond,
				Max:   7540 * time.Millisecond,
			})
		})
	})
})

func MakeHealthEndpointRequest(client *httpclient.HTTPClient, serverAddress string, hr chan<- Result) {
	startTime := time.Now()
	resp, err := secureGetHealthEndpoint(client, serverAddress)
	responseTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("Error hitting health endpoint: %s\n", err.Error())
		hr <- Result{status: http.StatusRequestTimeout, time: time.Now().Unix(), metricName: "response_time", value: responseTime}
	} else {
		hr <- Result{status: resp.StatusCode, time: time.Now().Unix(), metricName: "response_time", value: responseTime}
	}
}

func setupSecureGet() *httpclient.HTTPClient {
	// Load client cert
	cert, err := tls.LoadX509KeyPair("../healthcheck/assets/test_certs/test_client.pem", "../healthcheck/assets/test_certs/test_client.key")
	Expect(err).NotTo(HaveOccurred())

	// Load CA cert
	caCert, err := ioutil.ReadFile("../healthcheck/assets/test_certs/test_ca.pem")
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard)

	return healthclient.NewHealthClient(caCert, cert, logger)
}

func secureGetHealthEndpoint(client *httpclient.HTTPClient, serverAddress string) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s/health", serverAddress))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

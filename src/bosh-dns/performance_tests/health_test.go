package performance_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-dns/tlsclient"
)

var _ = Describe("Health Server", func() {
	var (
		serverAddress     = "127.0.0.2:8853"
		durationInSeconds = 60 * 10
		workers           = 10
		requestsPerSecond = 400
	)

	BeforeEach(func() {
		setupServers()
	})

	AfterEach(func() {
		shutdownServers()
	})

	TestHealthPerformance := func(timeThresholds TimeThresholds, vitalsThresholds VitalsThresholds) {
		httpClient := setupSecureGet()

		PerformanceTest{
			Application: "health",
			Context:     "health",

			Workers:           workers,
			RequestsPerSecond: requestsPerSecond,

			ServerPID: healthSessions[0].Command.Process.Pid,

			TimeThresholds:   timeThresholds,
			VitalsThresholds: vitalsThresholds,

			SuccessStatus: http.StatusOK,

			WorkerFunc: func(resultChan chan<- Result) {
				MakeHealthEndpointRequest(httpClient, serverAddress, resultChan)
			},
		}.Setup().TestPerformance(durationInSeconds, "health")
	}

	Describe("health server performance", func() {
		It("handles requests quickly", func() {
			TestHealthPerformance(healthTimeThresholds(), healthVitalsThresholds())
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
	caCert, err := os.ReadFile("../healthcheck/assets/test_certs/test_ca.pem")
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, io.Discard)

	client, err := tlsclient.New("health.bosh-dns", caCert, cert, 5*time.Second, logger)
	Expect(err).NotTo(HaveOccurred())
	return client
}

func secureGetHealthEndpoint(client *httpclient.HTTPClient, serverAddress string) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s/health", serverAddress))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

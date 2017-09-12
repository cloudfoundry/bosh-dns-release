package main_test

import (
	"bosh-dns/healthcheck/healthclient"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Health struct {
	State string `json:"state"`
}

var _ = Describe("HealthCheck server", func() {
	var (
		status string
		logger boshlog.Logger
	)

	BeforeEach(func() {
		status = "running"
		logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard)
	})

	Describe("/health", func() {
		JustBeforeEach(func() {
			healthRaw, err := json.Marshal(Health{State: status})
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(healthFile.Name(), healthRaw, 0777)
			Expect(err).ToNot(HaveOccurred())
		})

		It("reject non-TLS connections", func() {
			client := &http.Client{}
			resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", configPort))

			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
		})

		Describe("When the vm is healthy", func() {
			It("returns healthy json output", func() {
				client, err := healthclient.NewHealthClientFromFiles(
					"assets/test_certs/test_ca.pem",
					"assets/test_certs/test_client.pem",
					"assets/test_certs/test_client.key", logger)
				Expect(err).NotTo(HaveOccurred())

				respData, err := secureGetRespBody(client, configPort)
				Expect(err).ToNot(HaveOccurred())

				var respJson map[string]string
				err = json.Unmarshal(respData, &respJson)
				Expect(err).ToNot(HaveOccurred())

				Expect(respJson).To(Equal(map[string]string{
					"state": "running",
				}))
			})
		})

		Describe("When the vm is unhealthy", func() {
			BeforeEach(func() {
				status = "stopped"
			})

			It("returns unhealthy json output", func() {
				client, err := healthclient.NewHealthClientFromFiles(
					"assets/test_certs/test_ca.pem",
					"assets/test_certs/test_client.pem",
					"assets/test_certs/test_client.key", logger)
				Expect(err).NotTo(HaveOccurred())

				respData, err := secureGetRespBody(client, configPort)
				Expect(err).ToNot(HaveOccurred())

				var respJson map[string]string
				err = json.Unmarshal(respData, &respJson)
				Expect(err).ToNot(HaveOccurred())

				Expect(respJson).To(Equal(map[string]string{
					"state": "stopped",
				}))
			})
		})

		It("should reject a client cert with the wrong root CA", func() {
			client, err := healthclient.NewHealthClientFromFiles(
				"assets/test_certs/test_fake_ca.pem",
				"assets/test_certs/test_fake_client.pem",
				"assets/test_certs/test_client.key", logger)
			Expect(err).NotTo(HaveOccurred())

			_, err = secureGetRespBody(client, configPort)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))
		})

		It("should reject a client cert with the wrong CN", func() {
			client, err := healthclient.NewHealthClientFromFiles(
				"assets/test_certs/test_ca.pem",
				"assets/test_certs/test_wrong_cn_client.pem",
				"assets/test_certs/test_client.key", logger)
			Expect(err).NotTo(HaveOccurred())

			resp, err := secureGet(client, configPort)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(BeNumerically(">=", 400))
			Expect(resp.StatusCode).To(BeNumerically("<", 500))

			respBody, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(respBody).To(Equal([]byte("TLS certificate common name does not match")))
		})
	})
})

func waitForServer(port int) error {
	var err error
	for i := 0; i < 20; i++ {
		var c net.Conn
		c, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", strconv.Itoa(port)))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_ = c.Close()
		return nil
	}

	return err //errors.New("dns server failed to start")
}

func secureGetRespBody(client *httpclient.HTTPClient, port int) ([]byte, error) {
	resp, err := secureGet(client, port)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func secureGet(client *httpclient.HTTPClient, port int) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/health", port))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

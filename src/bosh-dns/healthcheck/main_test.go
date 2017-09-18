package main_test

import (
	"bosh-dns/healthcheck/healthclient"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"path/filepath"

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

			startServer()
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

		Context("when a health executable exists", func() {
			Describe("when the vm is healthy and the job health executable reports healthy", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(filepath.Join(healthExecutableDir, "good.ps1"), []byte("#!/bin/bash\nexit 0"), 0700)
					Expect(err).ToNot(HaveOccurred())
				})

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

			Describe("when the vm is healthy, but the job health executable reports unhealthy", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(filepath.Join(healthExecutableDir, "bad.ps1"), []byte("#!/bin/bash\nexit 1"), 0700)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns unhealthy json output", func() {
					client, err := healthclient.NewHealthClientFromFiles(
						"assets/test_certs/test_ca.pem",
						"assets/test_certs/test_client.pem",
						"assets/test_certs/test_client.key", logger)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() map[string]string {
						respData, err := secureGetRespBody(client, configPort)
						Expect(err).ToNot(HaveOccurred())
						var respJson map[string]string
						err = json.Unmarshal(respData, &respJson)
						Expect(err).ToNot(HaveOccurred())
						return respJson
					}, time.Second*2).Should(Equal(map[string]string{
						"state": "job-health-executable-fail",
					}))
				})
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

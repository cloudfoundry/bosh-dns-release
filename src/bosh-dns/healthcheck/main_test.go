package main_test

import (
	"bosh-dns/tlsclient"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Health struct {
	State      string            `json:"state"`
	GroupState map[string]string `json:"group_state,omitempty"`
}

var _ = Describe("HealthCheck server", func() {
	var (
		jobADir string
		logger  boshlog.Logger
		status  string
	)

	BeforeEach(func() {
		status = "running"
		logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, io.Discard)

		jobADir = filepath.Join(jobsDir, "job-a")
		err := os.MkdirAll(jobADir, 0777)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("/health", func() {
		var client *httpclient.HTTPClient

		JustBeforeEach(func() {
			healthRaw, err := json.Marshal(Health{State: status})
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(healthFile.Name(), healthRaw, 0777)
			Expect(err).ToNot(HaveOccurred())

			startServer()

			client, err = tlsclient.NewFromFiles(
				"health.bosh-dns",
				"assets/test_certs/test_ca.pem",
				"assets/test_certs/test_client.pem",
				"assets/test_certs/test_client.key",
				5*time.Second,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("reject non-TLS connections", func() {
			client := &http.Client{}
			resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", configPort))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})

		Describe("when the vm is healthy", func() {
			It("returns healthy json output", func() {
				resp := secureGetRespBody(client, configPort)
				Expect(resp.State).To(Equal("running"))
			})
		})

		Context("when groups are present", func() {
			BeforeEach(func() {
				err := os.MkdirAll(filepath.Join(jobADir, ".bosh"), 0777)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(jobADir, ".bosh", "links.json"), []byte(`[{"link":"service","group":"1"},{"group":"i-am-a-group"}]`), 0700)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(filepath.Join(jobADir, healthExecutablePath), []byte("#!/bin/bash\nexit 0"), 0700)
				Expect(err).ToNot(HaveOccurred())

				jobBDir := filepath.Join(jobsDir, "job-b")
				err = os.MkdirAll(filepath.Join(jobBDir, ".bosh"), 0777)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(jobBDir, ".bosh", "links.json"), []byte(`[{"link":"service","group":"2"}]`), 0700)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(filepath.Join(jobBDir, healthExecutablePath), []byte("#!/bin/bash\nexit 1"), 0700)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns group health status in the json output", func() {
				resp := secureGetRespBody(client, configPort)
				Expect(resp.State).To(Equal("failing"))
				Expect(resp.GroupState).To(Equal(map[string]string{
					"1":            "running",
					"i-am-a-group": "running",
					"2":            "failing",
				}))
			})
		})

		Context("when a health executable exists", func() {
			Describe("when the vm is healthy and the job health executable reports healthy", func() {
				BeforeEach(func() {
					err := os.WriteFile(filepath.Join(jobADir, healthExecutablePath), []byte("#!/bin/bash\nexit 0"), 0700)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns healthy json output", func() {
					resp := secureGetRespBody(client, configPort)
					Expect(resp.State).To(Equal("running"))
				})
			})

			Describe("when the vm is healthy, but the job health executable reports unhealthy", func() {
				BeforeEach(func() {
					err := os.WriteFile(filepath.Join(jobADir, healthExecutablePath), []byte("#!/bin/bash\nexit 1"), 0700)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns unhealthy json output", func() {
					Eventually(func() string {
						return secureGetRespBody(client, configPort).State
					}, time.Second*2).Should(Equal("failing"))
				})
			})
		})

		Describe("when the vm is unhealthy", func() {
			BeforeEach(func() {
				status = "failing"
			})

			It("returns unhealthy json output", func() {
				resp := secureGetRespBody(client, configPort)
				Expect(resp.State).To(Equal("failing"))
			})
		})

		It("should reject a client cert with the wrong root CA", func() {
			client, err := tlsclient.NewFromFiles(
				"health.bosh-dns",
				"assets/test_certs/test_fake_ca.pem",
				"assets/test_certs/test_fake_client.pem",
				"assets/test_certs/test_client.key",
				5*time.Second,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = secureGet(client, configPort)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))
		})

		It("should reject a client cert with the wrong CN", func() {
			client, err := tlsclient.NewFromFiles(
				"health.bosh-dns",
				"assets/test_certs/test_ca.pem",
				"assets/test_certs/test_wrong_cn_client.pem",
				"assets/test_certs/test_client.key",
				5*time.Second,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())

			resp, err := secureGet(client, configPort)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(BeNumerically(">=", 400))
			Expect(resp.StatusCode).To(BeNumerically("<", 500))

			respBody, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(respBody).To(Equal([]byte("TLS certificate common name does not match")))
		})
	})
})

func secureGetRespBody(client *httpclient.HTTPClient, port int) Health {
	resp, err := secureGet(client, port)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	data, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	var healthResp Health
	err = json.Unmarshal(data, &healthResp)
	Expect(err).ToNot(HaveOccurred())

	return healthResp
}

func secureGet(client *httpclient.HTTPClient, port int) (*http.Response, error) {
	return client.Get(fmt.Sprintf("https://127.0.0.1:%d/health", port))
}

package config_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"os"

	"bosh-dns/dns/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		listenAddress   string
		listenPort      int
		timeout         string
		recursorTimeout string

		healthCAFile            string
		healthCertificateFile   string
		upcheckInterval         string
		healthPort              int
		healthMaxTrackedQueries int
		healthPrivateKeyFile    string
		upcheckDomains          []string
	)

	BeforeEach(func() {
		rand.Seed(time.Now().Unix())

		listenAddress = fmt.Sprintf("192.168.1.%d", rand.Int31n(256))
		listenPort = rand.Int()
		timeout = fmt.Sprintf("%vs", rand.Int31n(16))
		recursorTimeout = fmt.Sprintf("%vs", rand.Int31n(16))
		healthPort = 2345
		healthMaxTrackedQueries = 50
		healthCertificateFile = "/etc/certificate"
		healthPrivateKeyFile = "/etc/private_key"
		healthCAFile = "/etc/ca"
		upcheckInterval = fmt.Sprintf("%vs", rand.Int31n(13))
		upcheckDomains = []string{"upcheck.domain.", "health2.bosh."}
	})

	It("returns config from a config file", func() {
		configContents, err := json.Marshal(map[string]interface{}{
			"address":          listenAddress,
			"port":             listenPort,
			"timeout":          timeout,
			"recursor_timeout": recursorTimeout,
			"upcheck_domains":  upcheckDomains,
			"health": map[string]interface{}{
				"enabled":             true,
				"port":                healthPort,
				"certificate_file":    healthCertificateFile,
				"private_key_file":    healthPrivateKeyFile,
				"ca_file":             healthCAFile,
				"check_interval":      upcheckInterval,
				"max_tracked_queries": healthMaxTrackedQueries,
			},
			"cache": map[string]interface{}{
				"enabled": true,
			},
			"handlers": []map[string]interface{}{{
				"domain": "some.tld.",
				"source": map[string]interface{}{
					"type": "http",
					"url":  "http.server.address",
				},
			}},
		})
		configFilePath := writeConfigFile(string(configContents))

		timeoutDuration, err := time.ParseDuration(timeout)
		Expect(err).ToNot(HaveOccurred())

		recursorTimeoutDuration, err := time.ParseDuration(recursorTimeout)
		Expect(err).ToNot(HaveOccurred())

		upcheckIntervalDuration, err := time.ParseDuration(upcheckInterval)
		Expect(err).ToNot(HaveOccurred())

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(dnsConfig).To(Equal(config.Config{
			Address:         listenAddress,
			Port:            listenPort,
			Timeout:         config.DurationJSON(timeoutDuration),
			RecursorTimeout: config.DurationJSON(recursorTimeoutDuration),
			UpcheckDomains:  []string{"upcheck.domain.", "health2.bosh."},
			Health: config.HealthConfig{
				Enabled:           true,
				Port:              healthPort,
				CertificateFile:   healthCertificateFile,
				PrivateKeyFile:    healthPrivateKeyFile,
				CAFile:            healthCAFile,
				CheckInterval:     config.DurationJSON(upcheckIntervalDuration),
				MaxTrackedQueries: healthMaxTrackedQueries,
			},
			Cache: config.Cache{
				Enabled: true,
			},
			Handlers: []config.Handler{
				{
					Domain: "some.tld.",
					Source: config.Source{
						Type: "http",
						URL:  "http.server.address",
					},
				},
			},
		}))
	})

	It("returns error if reading config file fails", func() {
		bogusPath := "some-bogus-path"
		_, err := config.LoadFromFile(bogusPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("returns error if the config is not json", func() {
		configFilePath := writeConfigFile(`{`)

		_, err := config.LoadFromFile(configFilePath)
		Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
	})

	It("returns error if port is not found", func() {
		configFilePath := writeConfigFile(`{"address": "127.0.0.1"}`)

		_, err := config.LoadFromFile(configFilePath)
		Expect(err).To(MatchError("port is required"))
	})

	Context("recursor_timeout", func() {
		It("defaults the recursor_timeout when not specified", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.RecursorTimeout).To(Equal(config.DurationJSON(2 * time.Second)))
		})
	})

	Context("records_file", func() {
		It("allows configuring the path", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "records_file": "/some/path"}`)
			dnsConfig, err := config.LoadFromFile(configFilePath)

			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.RecordsFile).To(Equal("/some/path"))
		})
	})

	Context("health.max_tracked_queries", func() {
		It("defaults to 2000", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Health.MaxTrackedQueries).To(Equal(2000))
		})
	})

	Context("timeout", func() {
		It("defaults timeout when not specified", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Timeout).To(Equal(config.DurationJSON(5 * time.Second)))
		})
	})

	Context("timeouts", func() {
		It("returns error if a timeout is not a valid time duration", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": "something"}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("time: invalid duration something"))
		})

		It("returns error if a timeout cannot be parsed", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": %%}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
		})
	})

	Context("configurable recursors", func() {
		It("allows multiple recursors to be configured with default port of 53", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursors": ["8.8.8.8","10.244.4.4:9700"]}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Recursors).To(ContainElement("8.8.8.8:53"))
			Expect(dnsConfig.Recursors).To(ContainElement("10.244.4.4:9700"))
		})

		It("defaults to no recursors", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(dnsConfig.Recursors)).To(Equal(0))
		})

		It("returns an error if the recursor address is malformed", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursors": ["::::::::::::"]}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("too many colons in address"))
			Expect(err.Error()).To(ContainSubstring("::::::::::::"))
		})
	})
})

func writeConfigFile(json string) string {
	configFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(json))
	Expect(err).NotTo(HaveOccurred())
	return configFile.Name()
}

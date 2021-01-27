package config_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"os"

	"bosh-dns/dns/config"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		addressesFileGlob       string
		aliasesFileGlob         string
		apiCAFile               string
		apiCertificateFile      string
		apiPrivateKeyFile       string
		handlersFileGlob        string
		healthCAFile            string
		healthCertificateFile   string
		healthMaxTrackedQueries int
		healthPort              int
		healthPrivateKeyFile    string
		listenAPIPort           int
		listenAddress           string
		listenPort              int
		logLevel                string
		recursorTimeout         string
		requestTimeout          string
		timeout                 string
		upcheckDomains          []string
		upcheckInterval         string
		synchronousCheckTimeout string
		metricsAddress          string
		metricsPort             int
		logFormat               string
	)

	BeforeEach(func() {
		rand.Seed(time.Now().Unix())

		addressesFileGlob = "/addresses/*/glob"
		aliasesFileGlob = "/aliases/*/glob"
		apiCAFile = "/api/ca"
		apiCertificateFile = "/api/cert"
		apiPrivateKeyFile = "/api/key"
		handlersFileGlob = "/handlers/*/glob"
		healthCAFile = "/etc/ca"
		healthCertificateFile = "/etc/certificate"
		healthMaxTrackedQueries = 50
		healthPort = 2345
		healthPrivateKeyFile = "/etc/private_key"
		listenAPIPort = rand.Int()
		listenAddress = fmt.Sprintf("192.168.1.%d", rand.Int31n(256))
		listenPort = rand.Int()
		logLevel = boshlog.AsString(boshlog.LevelDebug)
		recursorTimeout = fmt.Sprintf("%vs", rand.Int31n(16))
		requestTimeout = fmt.Sprintf("%vs", rand.Int31n(16))
		timeout = fmt.Sprintf("%vs", rand.Int31n(16))
		upcheckDomains = []string{"upcheck.domain.", "health2.bosh."}
		upcheckInterval = fmt.Sprintf("%vs", rand.Int31n(13))
		synchronousCheckTimeout = "1s"
		metricsPort = 53088
		metricsAddress = "127.0.0.1"
		logFormat = "rfc3339"
	})

	It("returns config from a config file", func() {
		configContents, err := json.Marshal(map[string]interface{}{
			"address":              listenAddress,
			"addresses_files_glob": addressesFileGlob,
			"alias_files_glob":     aliasesFileGlob,
			"handlers_files_glob":  handlersFileGlob,
			"port":                 listenPort,
			"log_level":            logLevel,
			"recursor_timeout":     recursorTimeout,
			"request_timeout":      requestTimeout,
			"excluded_recursors":   []string{"169.254.169.254", "169.10.10.10:1234"},
			"timeout":              timeout,
			"upcheck_domains":      upcheckDomains,
			"jobs_dir":             "/var/vcap/jobs",
			"api": map[string]interface{}{
				"port":             listenAPIPort,
				"certificate_file": apiCertificateFile,
				"private_key_file": apiPrivateKeyFile,
				"ca_file":          apiCAFile,
			},
			"health": map[string]interface{}{
				"enabled":                   true,
				"port":                      healthPort,
				"certificate_file":          healthCertificateFile,
				"private_key_file":          healthPrivateKeyFile,
				"ca_file":                   healthCAFile,
				"check_interval":            upcheckInterval,
				"max_tracked_queries":       healthMaxTrackedQueries,
				"synchronous_check_timeout": synchronousCheckTimeout,
			},
			"metrics": map[string]interface{}{
				"enabled": true,
				"address": metricsAddress,
				"port":    metricsPort,
			},
			"cache": map[string]interface{}{
				"enabled": true,
			},
			"internal_upcheck_domain": map[string]interface{}{
				"enabled":   true,
				"dns_query": "internal.test.query.",
			},
			"handlers": []map[string]interface{}{{
				"domain": "some.tld.",
				"cache": map[string]interface{}{
					"enabled": true,
				},
				"source": map[string]interface{}{
					"type": "http",
					"url":  "http.server.address",
				},
			}},
			"logging": map[string]interface{}{
				"format": map[string]interface{}{
					"timestamp": logFormat,
				},
			},
		})
		configFilePath := writeConfigFile(string(configContents))

		timeoutDuration, err := time.ParseDuration(timeout)
		Expect(err).ToNot(HaveOccurred())

		requestTimeoutDuration, err := time.ParseDuration(requestTimeout)
		Expect(err).ToNot(HaveOccurred())

		recursorTimeoutDuration, err := time.ParseDuration(recursorTimeout)
		Expect(err).ToNot(HaveOccurred())

		upcheckIntervalDuration, err := time.ParseDuration(upcheckInterval)
		Expect(err).ToNot(HaveOccurred())

		synchronousCheckTimeoutDuration, err := time.ParseDuration(synchronousCheckTimeout)
		Expect(err).ToNot(HaveOccurred())

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(dnsConfig).To(Equal(config.Config{
			Address:  listenAddress,
			Port:     listenPort,
			LogLevel: logLevel,
			API: config.APIConfig{
				Port:            listenAPIPort,
				CertificateFile: apiCertificateFile,
				PrivateKeyFile:  apiPrivateKeyFile,
				CAFile:          apiCAFile,
			},
			BindTimeout:        config.DurationJSON(timeoutDuration),
			RecursorRetryCount: 0,
			RequestTimeout:     config.DurationJSON(requestTimeoutDuration),
			RecursorTimeout:    config.DurationJSON(recursorTimeoutDuration),
			Recursors:          []string{},
			ExcludedRecursors:  []string{"169.254.169.254:53", "169.10.10.10:1234"},
			RecursorSelection:  "smart",
			UpcheckDomains:     []string{"upcheck.domain.", "health2.bosh."},
			AliasFilesGlob:     aliasesFileGlob,
			HandlersFilesGlob:  handlersFileGlob,
			AddressesFilesGlob: addressesFileGlob,
			JobsDir:            "/var/vcap/jobs",
			Health: config.HealthConfig{
				Enabled:                 true,
				Port:                    healthPort,
				CertificateFile:         healthCertificateFile,
				PrivateKeyFile:          healthPrivateKeyFile,
				CAFile:                  healthCAFile,
				CheckInterval:           config.DurationJSON(upcheckIntervalDuration),
				MaxTrackedQueries:       healthMaxTrackedQueries,
				SynchronousCheckTimeout: config.DurationJSON(synchronousCheckTimeoutDuration),
			},
			Metrics: config.MetricsConfig{
				Enabled: true,
				Address: metricsAddress,
				Port:    metricsPort,
			},
			Cache: config.Cache{
				Enabled: true,
			},
			InternalUpcheckDomain: config.InternalUpcheckDomain{
				Enabled:  true,
				DNSQuery: "internal.test.query.",
			},
			Logging: config.LoggingConfig{
				Format: config.FormatConfig{
					TimeStamp: logFormat,
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

	Context("recursor_selection", func() {
		It("allows configuring recursor selection to be serial", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursor_selection": "serial"}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.RecursorSelection).To(Equal("serial"))
		})

		It("allows configuring recursor selection to be smart", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursor_selection": "smart"}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.RecursorSelection).To(Equal("smart"))
		})

		It("defaults recursor selection to be smart", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53 }`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.RecursorSelection).To(Equal("smart"))
		})

		It("complains if you configure something besides smart or serial", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursor_selection": "wrong" }`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("invalid value for recursor_selection; expected 'serial' or 'smart'"))
		})

		It("recursor_retry_count default", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursor_selection": "smart" }`)

			c, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.RecursorRetryCount).To(Equal(0))
		})

		It("recursor_retry_count with value", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "recursor_selection": "smart", "recursor_retry_count": 3 }`)

			c, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.RecursorRetryCount).To(Equal(3))
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

	Context("metrics", func() {
		It("is disabled by default", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Metrics.Enabled).To(Equal(false))
		})

		It("has sensible defaults for port and address", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Metrics.Port).To(Equal(53088))
			Expect(dnsConfig.Metrics.Address).To(Equal("127.0.0.1"))
		})

		It("can override the port and address", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "metrics": {"address": "0.0.0.0", "port": 53089, "enabled": true}}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Metrics.Enabled).To(Equal(true))
			Expect(dnsConfig.Metrics.Address).To(Equal("0.0.0.0"))
			Expect(dnsConfig.Metrics.Port).To(Equal(53089))
		})
	})

	Context("timeout", func() {
		It("defaults timeout when not specified", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.BindTimeout).To(Equal(config.DurationJSON(5 * time.Second)))
		})
	})

	Context("timeouts", func() {
		It("returns error if a timeout is not a valid time duration", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": "something"}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError(MatchRegexp(`time\: invalid duration \"?something\"?`)))
		})

		It("returns error if a timeout cannot be parsed", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": %%}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
		})
	})

	It("defaults to no recursors", func() {
		configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(dnsConfig.Recursors)).To(Equal(0))
	})

	Context("LoggingFormat", func() {
		It("is case insensitive", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "logging":{"format": {"timestamp": "Rfc3339"}} }`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.UseRFC3339Formatting()).To(Equal(true))
		})
		It("can revert to legacy", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "logging":{"format": {"timestamp": "deprecated"}} }`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.UseRFC3339Formatting()).To(Equal(false))
		})
	})

	Context("AppendDefaultDNSPortIfMissing", func() {
		It("allows multiple recursors to be configured with default port of 53", func() {
			recursors, err := config.AppendDefaultDNSPortIfMissing([]string{"8.8.8.8", "10.244.4.4:9700", "2001:db8::1", "[2001:db8::1]:1234"})
			Expect(err).NotTo(HaveOccurred())

			Expect(recursors).To(ContainElement("8.8.8.8:53"))
			Expect(recursors).To(ContainElement("10.244.4.4:9700"))
			Expect(recursors).To(ContainElement("[2001:db8::1]:53"))
			Expect(recursors).To(ContainElement("[2001:db8::1]:1234"))
		})

		It("returns an error if the recursor address is malformed", func() {
			_, err := config.AppendDefaultDNSPortIfMissing([]string{"::::::::::::"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Invalid IP address"))
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

package config_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"os"

	"github.com/cloudfoundry/dns-release/src/dns/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		listenAddress   string
		listenPort      int
		timeout         string
		recursorTimeout string
	)

	BeforeEach(func() {
		rand.Seed(time.Now().Unix())

		listenAddress = fmt.Sprintf("192.168.1.%d", rand.Int31n(256))
		listenPort = rand.Int()
		timeout = fmt.Sprintf("%vs", rand.Int31n(16))
		recursorTimeout = fmt.Sprintf("%vs", rand.Int31n(16))
	})

	It("returns config from a config file", func() {
		configFilePath := writeConfigFile(fmt.Sprintf(`{
		  "address": "%s",
		  "port": %d,
		  "timeout": "%s",
		  "recursor_timeout": "%s",
		  "healthcheck_domains": %s
		}`, listenAddress, listenPort, timeout, recursorTimeout, `["healthcheck.domain.","health2.bosh."]`))

		timeoutDuration, err := time.ParseDuration(timeout)
		Expect(err).ToNot(HaveOccurred())

		recursorTimeoutDuration, err := time.ParseDuration(recursorTimeout)
		Expect(err).ToNot(HaveOccurred())

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(dnsConfig).To(Equal(config.Config{
			Address:            listenAddress,
			Port:               listenPort,
			Timeout:            config.Timeout(timeoutDuration),
			RecursorTimeout:    config.Timeout(recursorTimeoutDuration),
			HealthcheckDomains: []string{"healthcheck.domain.", "health2.bosh."},
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

			Expect(dnsConfig.RecursorTimeout).To(Equal(config.Timeout(2 * time.Second)))
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

	Context("timeout", func() {
		It("defaults timeout when not specified", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Timeout).To(Equal(config.Timeout(5 * time.Second)))
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
			Expect(err).To(MatchError("too many colons in address ::::::::::::"))
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

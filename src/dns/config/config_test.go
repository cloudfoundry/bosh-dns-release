package config_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/cloudfoundry/dns-release/src/dns/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		listenAddress string
		listenPort    int
		timeout       string
	)

	BeforeEach(func() {
		rand.Seed(time.Now().Unix())

		listenAddress = fmt.Sprintf("192.168.1.%d", rand.Int31n(256))
		listenPort = rand.Int()
		timeout = fmt.Sprintf("%vs", rand.Int31n(16))
	})

	It("returns config from a config file", func() {
		configFilePath := writeConfigFile(fmt.Sprintf(`{
		  "address": "%s",
		  "port": %d,
		  "timeout": "%s"
		}`, listenAddress, listenPort, timeout))

		timeoutDuration, err := time.ParseDuration(timeout)
		Expect(err).ToNot(HaveOccurred())

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(dnsConfig).To(Equal(config.Config{
			Address: listenAddress,
			Port:    listenPort,
			Timeout: config.Timeout(timeoutDuration),
		}))
	})

	It("returns error if reading config file fails", func() {
		bogusPath := "some-bogus-path"
		_, err := config.LoadFromFile(bogusPath)
		Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("%s: no such file or directory", bogusPath))))
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

	Context("configurable timeout", func() {
		It("defaults timeout when not specified", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53}`)

			dnsConfig, err := config.LoadFromFile(configFilePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(dnsConfig.Timeout).To(Equal(config.Timeout(5 * time.Second)))
		})

		It("returns error if timeout is not a valid time duration", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": "something"}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("time: invalid duration something"))
		})

		It("returns error if timeout cannot be parsed", func() {
			configFilePath := writeConfigFile(`{"address": "127.0.0.1", "port": 53, "timeout": %%}`)

			_, err := config.LoadFromFile(configFilePath)
			Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
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

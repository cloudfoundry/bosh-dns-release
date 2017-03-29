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
	)

	BeforeEach(func() {
		rand.Seed(time.Now().Unix())

		listenAddress = fmt.Sprintf("192.168.1.%d", rand.Int31n(256))
		listenPort = rand.Int()
	})

	It("returns config from a config file", func() {
		configFilePath := writeConfigFile(fmt.Sprintf(`{
			"dns": {
				"address": "%s",
				"port": %d
			}
		}`, listenAddress, listenPort))

		dnsConfig, err := config.LoadFromFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(dnsConfig).To(Equal(config.Config{
			DNS: config.DNSConfig{
				Address: listenAddress,
				Port:    listenPort,
			},
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
		configFilePath := writeConfigFile(`{"dns": {"address": "127.0.0.1"}}`)

		_, err := config.LoadFromFile(configFilePath)
		Expect(err).To(MatchError("port is required"))
	})
})

func writeConfigFile(json string) string {
	configFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(json))
	Expect(err).NotTo(HaveOccurred())
	return configFile.Name()
}

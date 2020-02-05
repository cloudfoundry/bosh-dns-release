package performance_test

import (
	"bosh-dns/dns/config"
	"bosh-dns/healthcheck/healthserver"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Performance Tests")
}

var (
	healthSessions []*gexec.Session
	dnsSession     *gexec.Session

	healthServerPath string
	healthPort       = 8853
	dnsPort          = 9953
	apiPort          = 10053
)

func setupServers() {
	healthSessions = []*gexec.Session{}
	var err error
	healthServerPath, err = gexec.Build("bosh-dns/healthcheck")
	Expect(err).NotTo(HaveOccurred())

	dnsServerPath, err := gexec.Build("bosh-dns/dns")
	Expect(err).NotTo(HaveOccurred())

	SetDefaultEventuallyTimeout(2 * time.Second)
	SetDefaultEventuallyPollingInterval(500*time.Millisecond)

	for i := 2; i <= 102; i++ {
		startHealthServer(fmt.Sprintf("127.0.0.%d", i))
	}

	dnsConfigFile, err := ioutil.TempFile("", "config.json")
	Expect(err).ToNot(HaveOccurred())

	dnsConfigContents, err := json.Marshal(map[string]interface{}{
		"address": "127.0.0.2",
		"port":    dnsPort,
		"api": map[string]interface{}{
			"port":             apiPort,
			"ca_file":          "../dns/api/assets/test_certs/test_ca.pem",
			"certificate_file": "../dns/api/assets/test_certs/test_server.pem",
			"private_key_file": "../dns/api/assets/test_certs/test_server.key",
		},
		"records_file":     "assets/records.json",
		"alias_files_glob": "assets/aliases.json",
		"upcheck_domains":  []string{"upcheck.bosh-dns."},
		"recursors":        []string{"34.194.75.123"},
		"recursor_timeout": "2s",
		"health": map[string]interface{}{
			"enabled":          true,
			"port":             healthPort,
			"ca_file":          "../healthcheck/assets/test_certs/test_ca.pem",
			"certificate_file": "../healthcheck/assets/test_certs/test_client.pem",
			"private_key_file": "../healthcheck/assets/test_certs/test_client.key",
			"check_interval":   "20s",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(dnsConfigFile.Name(), []byte(dnsConfigContents), 0666)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(healthPort)).To(Succeed())

	cmd := exec.Command(dnsServerPath, "--config="+dnsConfigFile.Name())
	dnsSession, err = gexec.Start(cmd, nil, nil)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(dnsPort)).To(Succeed())

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("2d111fee-966f-48a1-95d0-0d57bad45a08.dns.default.cf.bosh", dns.TypeA)
	for i := 0; i < 10; i++ {
		r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.2:%d", dnsPort))
		if err == nil && r.Rcode == dns.RcodeSuccess {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForServer(port int) error {
	var err error
	for i := 0; i < 20; i++ {
		var c net.Conn
		c, err = net.Dial("tcp", fmt.Sprintf("127.0.0.2:%s", strconv.Itoa(port)))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_ = c.Close()
		return nil
	}

	return err
}

func startHealthServer(addr string) {
	healthConfigFile, err := ioutil.TempFile("", "config.json")
	Expect(err).ToNot(HaveOccurred())

	healthFile, err := ioutil.TempFile("", "health.json")
	Expect(err).ToNot(HaveOccurred())

	healthConfigContents, err := json.Marshal(healthserver.HealthCheckConfig{
		Address:                  addr,
		Port:                     healthPort,
		CertificateFile:          "../healthcheck/assets/test_certs/test_server.pem",
		PrivateKeyFile:           "../healthcheck/assets/test_certs/test_server.key",
		CAFile:                   "../healthcheck/assets/test_certs/test_ca.pem",
		HealthFileName:           healthFile.Name(),
		HealthExecutableInterval: config.DurationJSON(time.Second),
	})
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(healthConfigFile.Name(), []byte(healthConfigContents), 0666)
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command(healthServerPath, healthConfigFile.Name())
	healthSession, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	healthSessions = append(healthSessions, healthSession)
	Consistently(healthSession).ShouldNot(gexec.Exit(),
		fmt.Sprintf(`
===========================================
 ATTENTION: on macOS you may need to run
    sudo ifconfig lo0 alias %s up
===========================================
`, addr),
	)
}

func shutdownServers() {
	for _, healthSession := range healthSessions {
		if healthSession != nil && healthSession.Command.Process != nil {
			Eventually(healthSession.Kill()).Should(gexec.Exit())
		}
	}

	if dnsSession != nil && dnsSession.Command.Process != nil {
		Eventually(dnsSession.Kill()).Should(gexec.Exit())
	}

	healthSessions = []*gexec.Session{}
	dnsSession = nil
}

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

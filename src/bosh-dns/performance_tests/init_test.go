package performance_test

import (
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
	healthSession *gexec.Session
	dnsSession    *gexec.Session
)

var _ = BeforeSuite(func() {
	healthServerPath, err := gexec.Build("bosh-dns/healthcheck")
	Expect(err).NotTo(HaveOccurred())

	dnsServerPath, err := gexec.Build("bosh-dns/dns")
	Expect(err).NotTo(HaveOccurred())

	SetDefaultEventuallyTimeout(2 * time.Second)

	healthConfigFile, err := ioutil.TempFile("", "config.json")
	Expect(err).ToNot(HaveOccurred())

	healthFile, err := ioutil.TempFile("", "health.json")
	Expect(err).ToNot(HaveOccurred())

	healthPort := 8853

	healthConfigContents, err := json.Marshal(healthserver.HealthCheckConfig{
		Port:            healthPort,
		CertificateFile: "../healthcheck/assets/test_certs/test_server.pem",
		PrivateKeyFile:  "../healthcheck/assets/test_certs/test_server.key",
		CAFile:          "../healthcheck/assets/test_certs/test_ca.pem",
		HealthFileName:  healthFile.Name(),
	})
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(healthConfigFile.Name(), []byte(healthConfigContents), 0666)
	Expect(err).ToNot(HaveOccurred())

	dnsConfigFile, err := ioutil.TempFile("", "config.json")
	Expect(err).ToNot(HaveOccurred())

	dnsPort := 9953

	dnsConfigContents, err := json.Marshal(map[string]interface{}{
		"address":          "127.0.0.1",
		"port":             dnsPort,
		"records_file":     "assets/records.json",
		"alias_files_glob": "assets/aliases.json",
		"upcheck_domains":  []string{"upcheck.bosh-dns."},
		"recursors":        []string{"8.8.8.8"},
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

	var cmd *exec.Cmd

	cmd = exec.Command(healthServerPath, healthConfigFile.Name())
	healthSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(healthPort)).To(Succeed())

	cmd = exec.Command(dnsServerPath, "--config="+dnsConfigFile.Name())
	dnsSession, err = gexec.Start(cmd, nil, nil)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(dnsPort)).To(Succeed())

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("2d111fee-966f-48a1-95d0-0d57bad45a08.dns.default.cf.bosh", dns.TypeA)
	for i := 0; i < 10; i++ {
		r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", dnsPort))
		if err == nil && r.Rcode == dns.RcodeSuccess {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
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

	return err
}

var _ = AfterSuite(func() {
	if healthSession != nil && healthSession.Command.Process != nil {
		Eventually(healthSession.Kill()).Should(gexec.Exit())
	}

	if dnsSession != nil && dnsSession.Command.Process != nil {
		Eventually(dnsSession.Kill()).Should(gexec.Exit())
	}

	gexec.CleanupBuildArtifacts()
})

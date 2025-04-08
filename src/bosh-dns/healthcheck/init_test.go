package main_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	dnsconfig "bosh-dns/dns/config"
	"bosh-dns/healthconfig"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck")
}

var (
	cmd                  *exec.Cmd
	configFile           *os.File
	configPort           int
	healthFile           *os.File
	jobsDir              string
	pathToServer         string
	sess                 *gexec.Session
	tmpDir               string
	healthExecutablePath string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	serverPath, err := gexec.Build("bosh-dns/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)

	return []byte(serverPath)
}, func(data []byte) {
	pathToServer = string(data)
})

var _ = BeforeEach(func() {
	var err error

	tmpDir, err = os.MkdirTemp("", "bosh-dns")
	Expect(err).ToNot(HaveOccurred())

	configFile, err = os.CreateTemp(tmpDir, "config.json")
	Expect(err).ToNot(HaveOccurred())

	healthFile, err = os.CreateTemp(tmpDir, "health.json")
	Expect(err).ToNot(HaveOccurred())

	jobsDir, err = os.MkdirTemp(tmpDir, "job-metadata")
	Expect(err).ToNot(HaveOccurred())

	suiteConfig, _ := GinkgoConfiguration()
	configPort = 1234 + suiteConfig.ParallelProcess

	healthExecutablePath = "healthy"
	if runtime.GOOS == "windows" {
		healthExecutablePath = "healthy.ps1"
	}

	configContents, err := json.Marshal(healthconfig.HealthCheckConfig{
		CAFile:                   "assets/test_certs/test_ca.pem",
		CertificateFile:          "assets/test_certs/test_server.pem",
		HealthExecutableInterval: dnsconfig.DurationJSON(time.Millisecond),
		HealthExecutablePath:     healthExecutablePath,
		HealthFileName:           healthFile.Name(),
		JobsDir:                  jobsDir,
		Port:                     configPort,
		PrivateKeyFile:           "assets/test_certs/test_server.key",
	})
	Expect(err).NotTo(HaveOccurred())

	err = os.WriteFile(configFile.Name(), []byte(configContents), 0666)
	Expect(err).ToNot(HaveOccurred())
	Expect(configFile.Close()).To(Succeed())
})

var _ = AfterEach(func() {
	if cmd.Process != nil {
		Eventually(sess.Kill()).Should(gexec.Exit())
	}

	Expect(healthFile.Close()).To(Succeed())

	Expect(os.RemoveAll(tmpDir)).To(Succeed())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

func startServer() {
	var err error
	cmd = exec.Command(pathToServer, configFile.Name())
	sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(configPort)).To(Succeed())
}

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

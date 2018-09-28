package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	dnsconfig "bosh-dns/dns/config"
	"bosh-dns/healthcheck/healthserver"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck")
}

var (
	cmd                 *exec.Cmd
	configFile          *os.File
	configPort          int
	healthExecutableDir string
	healthFile          *os.File
	pathToServer        string
	sess                *gexec.Session
	tmpDir              string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	serverPath, err := gexec.Build("bosh-dns/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)

	return []byte(serverPath)
}, func(data []byte) {
	pathToServer = string(data)
})

var _ = BeforeEach(func() {
	var err error

	tmpDir, err = ioutil.TempDir("", "bosh-dns")
	Expect(err).ToNot(HaveOccurred())

	configFile, err = ioutil.TempFile(tmpDir, "config.json")
	Expect(err).ToNot(HaveOccurred())

	healthFile, err = ioutil.TempFile(tmpDir, "health.json")
	Expect(err).ToNot(HaveOccurred())

	healthExecutableDir, err = ioutil.TempDir(tmpDir, "health-executables")
	Expect(err).ToNot(HaveOccurred())

	configPort = 1234 + config.GinkgoConfig.ParallelNode

	configContents, err := json.Marshal(healthserver.HealthCheckConfig{
		CAFile:                   "assets/test_certs/test_ca.pem",
		CertificateFile:          "assets/test_certs/test_server.pem",
		HealthExecutableInterval: dnsconfig.DurationJSON(time.Millisecond),
		HealthExecutablesGlob:    filepath.Join(healthExecutableDir, "*"),
		HealthFileName:           healthFile.Name(),
		Port:                     configPort,
		PrivateKeyFile:           "assets/test_certs/test_server.key",
	})
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), []byte(configContents), 0666)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(tmpDir)).To(Succeed())

	if cmd.Process != nil {
		Eventually(sess.Kill()).Should(gexec.Exit())
	}
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

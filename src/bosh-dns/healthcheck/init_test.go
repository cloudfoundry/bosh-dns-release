package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

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
	pathToServer string
	sess         *gexec.Session
	cmd          *exec.Cmd
	healthFile   *os.File
	configFile   *os.File
	configPort   int
)

var _ = BeforeSuite(func() {
	var err error

	pathToServer, err = gexec.Build("bosh-dns/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)

	configFile, err = ioutil.TempFile("", "config.json")
	Expect(err).ToNot(HaveOccurred())

	healthFile, err = ioutil.TempFile("", "health.json")
	Expect(err).ToNot(HaveOccurred())

	configPort = 1234 + config.GinkgoConfig.ParallelNode

	configContents, err := json.Marshal(healthserver.HealthCheckConfig{
		Port:            configPort,
		CertificateFile: "assets/test_certs/test_server.pem",
		PrivateKeyFile:  "assets/test_certs/test_server.key",
		CAFile:          "assets/test_certs/test_ca.pem",
		HealthFileName:  healthFile.Name(),
	})
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), []byte(configContents), 0666)
	Expect(err).ToNot(HaveOccurred())

	// run the server
	cmd = exec.Command(pathToServer, configFile.Name())
	sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(configPort)).To(Succeed())
})

var _ = AfterSuite(func() {
	if cmd.Process != nil {
		if runtime.GOOS == "windows" {
			killcmd := exec.Command("powershell", "Taskkill", "/PID", string(cmd.Process.Pid), "/F")
			_, err := gexec.Start(killcmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

		} else {
			sess.Terminate()
			sess.Wait()
		}
	}

	gexec.CleanupBuildArtifacts()
})

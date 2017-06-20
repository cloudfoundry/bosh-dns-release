package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"encoding/json"
	. "github.com/cloudfoundry/dns-release/src/healthcheck"
	"io/ioutil"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck")
}

var (
	pathToServer string
	config       *HealthCheckConfig
	sess         *gexec.Session
	cmd          *exec.Cmd
)

var _ = BeforeSuite(func() {
	var err error

	pathToServer, err = gexec.Build("github.com/cloudfoundry/dns-release/src/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)

	// run the server
	configFile := "assets/test_server.json"
	cmd = exec.Command(pathToServer, configFile)
	sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	configRaw, err := ioutil.ReadFile(configFile)
	Expect(err).ToNot(HaveOccurred())

	config = &HealthCheckConfig{}
	err = json.Unmarshal(configRaw, config)
	Expect(err).ToNot(HaveOccurred())

	Expect(waitForServer(config.Port)).To(Succeed())
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

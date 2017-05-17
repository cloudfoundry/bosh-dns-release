// +build linux darwin

package no_recursors_configured

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
	"encoding/json"
	"strings"
	"os/exec"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "recursors/no_recursors_configured")
}

var (
	boshBinaryPath           string
	boshDeployment           string
	cloudConfigTempFileName  string
	pathToTestRecursorServer string
	allDeployedInstances     []instanceInfo
	cmdRunner                system.CmdRunner
)

type instanceInfo struct {
	IP            string
	InstanceID    string
	InstanceGroup string
}

var _ = BeforeSuite(func() {
	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	boshDeployment = fmt.Sprintf("%s-no_recursors_configured", assertEnvExists("BOSH_DEPLOYMENT"))

	cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs("../../../../../ci/assets/manifest.yml")
	Expect(err).ToNot(HaveOccurred())
	disableOverridePath, err := filepath.Abs("no-recursors-configured.yml")
	Expect(err).ToNot(HaveOccurred())
	dnsReleasePath, err := filepath.Abs("../../../../../")
	Expect(err).ToNot(HaveOccurred())
	aliasProvidingPath, err := filepath.Abs("../../../dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	cloudConfigTempFileName = backupCloudConfig()

	updateCloudConfigWithOurLocalRecursor()

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("dns_release_path=%s", dnsReleasePath),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"-o", disableOverridePath,
		manifestPath,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))

	setupTestRecursor()

	allDeployedInstances = getInstanceInfos(boshBinaryPath)
})

var _ = AfterSuite(func() {
	restoreCloudConfig()

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "delete-deployment",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
})

func setupTestRecursor() {
	var err error
	pathToTestRecursorServer, err = gexec.Build("github.com/cloudfoundry/dns-release/src/acceptance_tests/test_recursor")
	Expect(err).NotTo(HaveOccurred())

}

func restoreCloudConfig() {
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "-n", "update-cloud-config", cloudConfigTempFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
}

func updateCloudConfigWithOurLocalRecursor() {
	removeRecursorAddressesOpsFile, err := filepath.Abs("add_test_dns_nameservers.yml")
	Expect(err).ToNot(HaveOccurred())
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "-n", "update-cloud-config", "-o", removeRecursorAddressesOpsFile, cloudConfigTempFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
}

func backupCloudConfig() string {
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "cloud-config")
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	cloudConfigTmp, err := ioutil.TempFile(os.TempDir(), "cloud-config")
	Expect(err).ToNot(HaveOccurred())
	ioutil.WriteFile(cloudConfigTmp.Name(), []byte(stdOut), os.ModePerm)
	return cloudConfigTmp.Name()
}

func assertEnvExists(envName string) string {
	val, found := os.LookupEnv(envName)
	if !found {
		Fail(fmt.Sprintf("Expected %s", envName))
	}
	return val
}


func getInstanceInfos(boshBinary string) []instanceInfo {
	cmd := exec.Command(boshBinary, "instances", "-d", boshDeployment, "--json")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 10*time.Second).Should(gexec.Exit(0))

	var response struct {
		Tables []struct {
			Rows []map[string]string
		}
	}

	out := []instanceInfo{}

	json.Unmarshal(session.Out.Contents(), &response)

	for _, row := range response.Tables[0].Rows {
		instanceStrings := strings.Split(row["instance"], "/")

		out = append(out, instanceInfo{
			IP:            row["ips"],
			InstanceGroup: instanceStrings[0],
			InstanceID:    instanceStrings[1],
		})
	}

	return out
}
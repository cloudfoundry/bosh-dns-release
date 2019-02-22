// +build linux darwin

package override_nameserver

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"path/filepath"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "override_nameserver/disabled")
}

var (
	boshBinaryPath string
	boshDeployment string
)

var _ = BeforeSuite(func() {
	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	boshDeployment = fmt.Sprintf("%s-override-nameserver", assertEnvExists("BOSH_DEPLOYMENT"))

	cmdRunner := system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs("../../../../test_yml_assets/manifests/dns-linux.yml")
	Expect(err).ToNot(HaveOccurred())
	defaultBindOpsPath, err := filepath.Abs("../../../../test_yml_assets/ops/use-dns-release-default-bind-and-alias-addresses.yml")
	Expect(err).ToNot(HaveOccurred())
	disableOverridePath, err := filepath.Abs("disable-override-nameserver.yml")
	Expect(err).ToNot(HaveOccurred())

	baseStemcell := assertEnvExists("BASE_STEMCELL")

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-o", defaultBindOpsPath,
		"-o", disableOverridePath,
		"--vars-store", "dns-creds.yml",
		manifestPath,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
})

var _ = AfterSuite(func() {
	cmdRunner := system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "delete-deployment",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
})

func assertEnvExists(envName string) string {
	val, found := os.LookupEnv(envName)
	if !found {
		Fail(fmt.Sprintf("Expected %s", envName))
	}
	return val
}

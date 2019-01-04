package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"fmt"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"testing"

	"github.com/onsi/gomega/gexec"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance")
}

var (
	pathToTestRecursorServer string
	pathToTestHTTPDNSServer  string
	allDeployedInstances     []helpers.InstanceInfo
	boshDeployment           string
	cmdRunner                system.CmdRunner
	cloudConfigTempFileName  string
	testTargetOS             string
	baseStemcell             string
)

var _ = BeforeSuite(func() {
	cloudConfigTempFileName = assertEnvExists("TEST_CLOUD_CONFIG_PATH")
	testTargetOS = assertEnvExists("TEST_TARGET_OS")
	baseStemcell = assertEnvExists("BASE_STEMCELL")
	boshDeployment = assertEnvExists("BOSH_DEPLOYMENT")

	var err error
	pathToTestRecursorServer, err = gexec.Build("bosh-dns/acceptance_tests/test_recursor")
	Expect(err).NotTo(HaveOccurred())

	pathToTestHTTPDNSServer, err = gexec.Build("bosh-dns/acceptance_tests/test_http_dns_server")
	Expect(err).NotTo(HaveOccurred())

	cmdRunner = system.NewExecCmdRunner(logger.NewLogger(logger.LevelDebug))
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func assertEnvExists(envName string) string {
	val, found := os.LookupEnv(envName)
	if !found {
		Fail(fmt.Sprintf("Expected %s", envName))
	}
	return val
}

func testManifestName() string {
	if testTargetOS == "windows" {
		return "manifests/dns-windows.yml"
	}

	return "manifests/dns-linux.yml"
}

func noRecursorsOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/no-recursors-configured-windows.yml"
	}

	return "ops/no-recursors-configured.yml"
}

func excludedRecursorsOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/add-excluded-recursors-windows.yml"
	}

	return "ops/add-excluded-recursors.yml"
}

func jsonServerAddress() string {
	if testTargetOS == "windows" {
		return "http://10.0.255.5:8081"
	}

	return "http://172.17.0.1:8081"
}

func setupLocalRecursorOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/add-test-dns-nameservers-windows.yml"
	}

	return "ops/add-test-dns-nameservers.yml"
}

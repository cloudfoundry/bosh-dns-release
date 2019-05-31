package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"fmt"

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
	allDeployedInstances    []helpers.InstanceInfo
	boshDeployment          string
	cloudConfigTempFileName string
	testTargetOS            string
	baseStemcell            string
)

var _ = BeforeSuite(func() {
	cloudConfigTempFileName = assertEnvExists("TEST_CLOUD_CONFIG_PATH")
	testTargetOS = assertEnvExists("TEST_TARGET_OS")
	baseStemcell = assertEnvExists("BASE_STEMCELL")
	boshDeployment = assertEnvExists("BOSH_DEPLOYMENT")

	deployTestRecursors()
	deployTestHTTPDNSServer()
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

func enableHealthManifestOps() string {
	if testTargetOS == "windows" {
		return "ops/manifest/enable-health-manifest-windows.yml"
	}

	return "ops/manifest/enable-health-manifest-linux.yml"
}

func noRecursorsOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/manifest/no-recursors-configured-windows.yml"
	}

	return "ops/manifest/no-recursors-configured.yml"
}

func excludedRecursorsOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/manifest/add-excluded-recursors-windows.yml"
	}

	return "ops/manifest/add-excluded-recursors.yml"
}

func configureSerialRecursorSelectionOpsFile() string {
	return "ops/manifest/configure-serial-recursor-selection.yml"
}

func configureSmartRecursorSelectionOpsFile() string {
	return "ops/manifest/configure-smart-recursor-selection.yml"
}

func configureRecursorOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/manifest/configure-recursor-windows.yml"
	}

	return "ops/manifest/configure-recursor.yml"
}

func excludeSpecificRecursor() string {
	return "ops/manifest/exclude-specific-recursor.yml"
}

func enableHTTPJSONEndpointsOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/manifest/enable-http-json-endpoints-windows.yml"
	}

	return "ops/manifest/enable-http-json-endpoints-linux.yml"
}

func setupLocalRecursorOpsFile() string {
	if testTargetOS == "windows" {
		return "ops/cloud-config/add-test-dns-nameservers-windows.yml"
	}

	return "ops/cloud-config/add-test-dns-nameservers.yml"
}

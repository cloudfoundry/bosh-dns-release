package acceptance_test

import (
	"fmt"

	"github.com/cloudfoundry/bosh-utils/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance")
}

var (
	pathToTestRecursorServer  string
	boshBinaryPath            string
	allDeployedInstances      []instanceInfo
	manifestName              string
	noRecursorsOpsFile        string
	setupLocalRecursorOpsFile string
	boshDeployment            string
	cmdRunner                 system.CmdRunner
	cloudConfigTempFileName   string
)

var _ = BeforeSuite(func() {
	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	boshDeployment = assertEnvExists("BOSH_DEPLOYMENT")
	manifestName = assertEnvExists("TEST_MANIFEST_NAME")
	noRecursorsOpsFile = assertEnvExists("NO_RECURSORS_OPS_FILE")
	setupLocalRecursorOpsFile = assertEnvExists("LOCAL_RECURSOR_OPS_FILE")
	cloudConfigTempFileName = assertEnvExists("TEST_CLOUD_CONFIG_PATH")

	var err error
	pathToTestRecursorServer, err = gexec.Build("github.com/cloudfoundry/dns-release/src/acceptance_tests/test_recursor")
	Expect(err).NotTo(HaveOccurred())
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

type instanceInfo struct {
	IP            string
	InstanceID    string
	InstanceGroup string
}

func getInstanceInfos(boshBinary string) []instanceInfo {
	cmd := exec.Command(boshBinary, "instances", "--json")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 20*time.Second).Should(gexec.Exit(0))

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

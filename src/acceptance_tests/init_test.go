package acceptance_test

import (
	"fmt"

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
	pathToTestRecursorServer string
	boshBinaryPath           string
	allDeployedInstances     []instanceInfo
	firstInstanceSlug        string
)

var _ = BeforeSuite(func() {
	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	assertEnvExists("BOSH_DEPLOYMENT")

	allDeployedInstances = getInstanceInfos(boshBinaryPath)
	firstInstanceSlug = fmt.Sprintf("%s/%s", allDeployedInstances[0].InstanceGroup, allDeployedInstances[0].InstanceID)

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

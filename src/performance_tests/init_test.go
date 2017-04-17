package performance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/cloudfoundry/gosigar"
	"os"
	"strings"
	"testing"
)

func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "performance")
}

var (
	boshBinaryPath string
)

var _ = BeforeSuite(func() {
	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	assertEnvExists("BOSH_DEPLOYMENT")
})

func assertEnvExists(envName string) string {
	val, found := os.LookupEnv(envName)
	if !found {
		Fail(fmt.Sprintf("Expected %s", envName))
	}
	return val
}

func GetPidFor(processName string) (int, bool) {
	pids := sigar.ProcList{}
	pids.Get()

	for _, pid := range pids.List {
		state := sigar.ProcState{}

		if err := state.Get(pid); err != nil {
			continue
		}

		if strings.Contains(state.Name, processName) {
			return pid, true
		}
	}

	return -1, false
}

package performance_test

import  (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/gosigar"
	"testing"
	"strings"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "performance")
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

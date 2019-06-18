package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	boshBinaryPath string
	cmdRunner      system.CmdRunner
)

func Bosh(args ...string) string {
	args = append(args, "-n")
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, args...)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	return stdOut
}

func BoshRunErrand(errandName, instanceSlug string) string {
	session, err := gexec.Start(exec.Command(
		boshBinaryPath, "-n",
		"run-errand", errandName,
		"--instance", instanceSlug,
	), ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session, time.Minute).Should(gexec.Exit(0))
	return string(session.Out.Contents())
}

type InstanceInfo struct {
	IP            string
	InstanceID    string
	InstanceGroup string
	Index         string
	ProcessState  string
}

func BoshInstances(deploymentName string) []InstanceInfo {
	output := Bosh("instances", "-d", deploymentName, "--details", "--json")

	var response struct {
		Tables []struct {
			Rows []map[string]string
		}
	}

	out := []InstanceInfo{}

	json.Unmarshal([]byte(output), &response)

	for _, row := range response.Tables[0].Rows {
		instanceStrings := strings.Split(row["instance"], "/")

		out = append(out, InstanceInfo{
			IP:            row["ips"],
			InstanceGroup: instanceStrings[0],
			InstanceID:    instanceStrings[1],
			Index:         row["index"],
			ProcessState:  row["process_state"],
		})
	}

	return out
}

func init() {
	defer ginkgo.GinkgoRecover()

	var found bool
	boshBinaryPath, found = os.LookupEnv("BOSH_BINARY_PATH")
	if !found {
		fmt.Fprintln(ginkgo.GinkgoWriter, "WARNING: No bosh binary path set; This may be ignored if not using helpers.Bosh")
	}
	cmdRunner = system.NewExecCmdRunner(logger.NewLogger(logger.LevelDebug))
}

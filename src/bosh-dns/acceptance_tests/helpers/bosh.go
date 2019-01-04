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
}

func BoshInstances() []InstanceInfo {
	output := Bosh("instances", "--details", "--json")

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
		})
	}

	return out
}

func init() {
	defer ginkgo.GinkgoRecover()

	boshBinaryPath = assertEnvExists("BOSH_BINARY_PATH")
	assertEnvExists("BOSH_CLIENT")
	assertEnvExists("BOSH_CLIENT_SECRET")
	assertEnvExists("BOSH_CA_CERT")
	assertEnvExists("BOSH_ENVIRONMENT")
	assertEnvExists("BOSH_DEPLOYMENT")

	cmdRunner = system.NewExecCmdRunner(logger.NewLogger(logger.LevelDebug))
}

func assertEnvExists(envName string) string {
	val, found := os.LookupEnv(envName)
	if !found {
		ginkgo.Fail(fmt.Sprintf("Expected %s", envName))
	}
	return val
}

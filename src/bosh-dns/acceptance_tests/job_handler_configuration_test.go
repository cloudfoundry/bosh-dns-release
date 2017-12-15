package acceptance_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler Configuration through Job Configuration File", func() {
	var (
		firstInstance       instanceInfo
		testRecursorSession *gexec.Session
	)

	BeforeEach(func() {
		var err error
		cmd := exec.Command(pathToTestRecursorServer, "9956")
		testRecursorSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

		manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", testManifestName()))
		Expect(err).ToNot(HaveOccurred())
		acceptanceTestReleasePath, err := filepath.Abs("dns-acceptance-release")
		Expect(err).ToNot(HaveOccurred())

		updateCloudConfigWithDefaultCloudConfig()

		stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
			"-n", "-d", boshDeployment, "deploy",
			"-v", fmt.Sprintf("name=%s", boshDeployment),
			"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
			"-v", fmt.Sprintf("acceptance_release_path=%s", acceptanceTestReleasePath),
			"-v", fmt.Sprintf("http_json_server_address=%s", jsonServerAddress()),
			manifestPath,
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
		allDeployedInstances = getInstanceInfos(boshBinaryPath)
		firstInstance = allDeployedInstances[0]
	})

	AfterEach(func() {
		testRecursorSession.Kill()
	})

	It("configures Bosh DNS handlers by producing a dns handlers json file", func() {
		cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A handler.internal.local @%s", firstInstance.IP), " ")...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		<-session.Exited
		Expect(session.ExitCode()).To(BeZero())

		output := string(session.Out.Contents())
		Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Expect(output).To(MatchRegexp("handler.internal.local.\\s+0\\s+IN\\s+A\\s+10\\.168\\.0\\.1"))
		Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
	})
})

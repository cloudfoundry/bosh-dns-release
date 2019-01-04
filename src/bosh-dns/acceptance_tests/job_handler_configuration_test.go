package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
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
		firstInstance       helpers.InstanceInfo
		testRecursorSession *gexec.Session
	)

	BeforeEach(func() {
		var err error
		cmd := exec.Command(pathToTestRecursorServer, "9956")
		testRecursorSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

		manifestPath := assetPath(testManifestName())
		acceptanceTestReleasePath, err := filepath.Abs("dns-acceptance-release")
		Expect(err).ToNot(HaveOccurred())

		updateCloudConfigWithDefaultCloudConfig()

		helpers.Bosh(
			"deploy",
			"-v", fmt.Sprintf("name=%s", boshDeployment),
			"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
			"-v", fmt.Sprintf("acceptance_release_path=%s", acceptanceTestReleasePath),
			"-v", fmt.Sprintf("http_json_server_address=%s", jsonServerAddress()),
			"--vars-store", "creds.yml",
			manifestPath,
		)

		allDeployedInstances = helpers.BoshInstances()
		firstInstance = allDeployedInstances[0]
	})

	AfterEach(func() {
		testRecursorSession.Kill()
	})

	It("configures Bosh DNS handlers by producing a dns handlers json file", func() {
		cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A handler.internal.local @%s", firstInstance.IP), " ")...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		output := string(session.Out.Contents())
		Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Expect(output).To(MatchRegexp("handler.internal.local.\\s+0\\s+IN\\s+A\\s+10\\.168\\.0\\.1"))
		Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
	})
})

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

var _ = Describe("HTTP JSON Server integration", func() {
	var (
		firstInstance  instanceInfo
		httpDNSSession *gexec.Session
	)

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			var err error
			cmd := exec.Command(pathToTestHTTPDNSServer)
			httpDNSSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

			manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/manifests/%s.yml", testManifestName()))
			Expect(err).ToNot(HaveOccurred())
			aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
			Expect(err).ToNot(HaveOccurred())
			enableHTTPJSONEndpointsPath, err := filepath.Abs("../test_yml_assets/ops/enable-http-json-endpoints.yml")
			Expect(err).ToNot(HaveOccurred())

			updateCloudConfigWithDefaultCloudConfig()

			stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
				"-n", "-d", boshDeployment, "deploy",
				"-o", enableHTTPJSONEndpointsPath,
				"-v", fmt.Sprintf("name=%s", boshDeployment),
				"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
				"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
				"-v", fmt.Sprintf("http_json_server_address=%s", jsonServerAddress()),
				"--vars-store", "creds.yml",
				manifestPath,
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
			allDeployedInstances = getInstanceInfos(boshBinaryPath)
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			httpDNSSession.Kill()
		})

		It("answers queries with the response from the http server", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A app-id.internal.local @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("app-id.internal.local.\\s+0\\s+IN\\s+A\\s+192\\.168\\.0\\.1"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})
})

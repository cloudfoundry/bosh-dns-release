package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"

	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/onsi/gomega/gexec"

	gomegadns "bosh-dns/gomega-dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP JSON Server integration", func() {
	var (
		firstInstance  helpers.InstanceInfo
		httpDNSSession *gexec.Session
	)

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			var err error
			cmd := exec.Command(pathToTestHTTPDNSServer)
			httpDNSSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			cmdRunner = system.NewExecCmdRunner(logger.NewLogger(logger.LevelDebug))

			manifestPath := assetPath(testManifestName())
			aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
			Expect(err).ToNot(HaveOccurred())
			enableHTTPJSONEndpointsPath := assetPath("ops/enable-http-json-endpoints.yml")

			updateCloudConfigWithDefaultCloudConfig()

			helpers.Bosh(
				"deploy",
				"-o", enableHTTPJSONEndpointsPath,
				"-v", fmt.Sprintf("name=%s", boshDeployment),
				"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
				"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
				"-v", fmt.Sprintf("http_json_server_address=%s", jsonServerAddress()),
				"--vars-store", "creds.yml",
				manifestPath,
			)

			allDeployedInstances = helpers.BoshInstances()
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			httpDNSSession.Kill()
		})

		It("answers queries with the response from the http server", func() {
			dnsResponse := helpers.Dig("app-id.internal.local.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": firstInstance.IP, "ttl": 0}),
			))
		})
	})
})

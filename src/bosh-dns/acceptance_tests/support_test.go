package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/tlsclient"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
)

func assetPath(name string) string {
	path, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s", name))
	Expect(err).ToNot(HaveOccurred())
	return path
}

func ensureRecursorIsDefinedByBoshAgent() {
	manifestPath := assetPath(testManifestName())
	disableOverridePath := assetPath(noRecursorsOpsFile())
	excludedRecursorsPath := assetPath(excludedRecursorsOpsFile())

	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithOurLocalRecursor()

	helpers.Bosh(
		"deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"-o", disableOverridePath,
		"-o", excludedRecursorsPath,
		"--vars-store", "creds.yml",
		manifestPath,
	)
	allDeployedInstances = helpers.BoshInstances()
}

func ensureRecursorIsDefinedByDnsRelease() {
	manifestPath := assetPath(testManifestName())

	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithDefaultCloudConfig()

	helpers.Bosh(
		"deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"--vars-store", "creds.yml",
		manifestPath,
	)
	allDeployedInstances = helpers.BoshInstances()
}

func updateCloudConfigWithOurLocalRecursor() {
	removeRecursorAddressesOpsFile := assetPath(setupLocalRecursorOpsFile())

	helpers.Bosh(
		"update-cloud-config",
		"-o", removeRecursorAddressesOpsFile,
		"-v", "network=director_network",
		cloudConfigTempFileName,
	)
}

func updateCloudConfigWithDefaultCloudConfig() {
	helpers.Bosh(
		"update-cloud-config",
		"-v", "network=director_network",
		cloudConfigTempFileName,
	)
}

func resolve(address, server string) []string {
	fmt.Println(strings.Split(fmt.Sprintf("+short %s @%s", address, server), " "))
	cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+short %s @%s", address, server), " ")...)
	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session).Should(gexec.Exit(0))

	return strings.Split(strings.TrimSpace(string(session.Out.Contents())), "\n")
}

func ensureHealthEndpointDeployed(extraOps ...string) {
	manifestPath := assetPath(testManifestName())

	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithDefaultCloudConfig()

	args := []string{
		"deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-v", "health_server_port=2345",
		"-o", assetPath("ops/enable-health-manifest-ops.yml"),
		"--vars-store", "creds.yml",
		manifestPath,
	}

	args = append(args, extraOps...)
	helpers.Bosh(args...)

	allDeployedInstances = helpers.BoshInstances()
}

func setupSecureGet() *httpclient.HTTPClient {
	clientCertificate := helpers.Bosh(
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/certificate",
	)

	clientPrivateKey := helpers.Bosh(
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/private_key",
	)

	caCert := helpers.Bosh(
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/ca",
	)

	cert, err := tls.X509KeyPair([]byte(clientCertificate), []byte(clientPrivateKey))
	Expect(err).NotTo(HaveOccurred())

	logger := logger.NewAsyncWriterLogger(logger.LevelDebug, ioutil.Discard)
	return tlsclient.New("health.bosh-dns", []byte(caCert), cert, logger)
}

type healthResponse struct {
	State      string            `json:"state"`
	GroupState map[string]string `json:"group_state"`
}

func secureGetRespBody(client *httpclient.HTTPClient, hostname string, port int) healthResponse {
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", hostname, port))
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	data, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	var respJson healthResponse
	err = json.Unmarshal(data, &respJson)
	Expect(err).ToNot(HaveOccurred())

	return respJson
}

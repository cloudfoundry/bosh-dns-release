package acceptance

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/gomega"

	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/tlsclient"
)

var (
	RecursorIPAddresses []string
)

func assetPath(name string) string {
	path, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s", name))
	Expect(err).ToNot(HaveOccurred())
	return path
}

func deployTestRecursors() {
	helpers.Bosh(
		"deploy",
		"-d", "test-recursor",
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		assetPath("manifests/recursor.yml"),
	)

	fetchRecursorIPAddresses()
}

func deployTestHTTPDNSServer() {
	helpers.Bosh(
		"deploy",
		"-d", "test-http-dns-server",
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		assetPath("manifests/http-dns-server.yml"),
	)
}

func fetchRecursorIPAddresses() {
	if RecursorIPAddresses == nil {
		for _, r := range helpers.BoshInstances("test-recursor") {
			RecursorIPAddresses = append(RecursorIPAddresses, r.IP)
		}

		Expect(RecursorIPAddresses).To(HaveLen(2),
			"We handle exactly two upstream recursors currently so this should change if you add more")
	}
}

func testHTTPDNSServerIPAddress() string {
	testHTTPDNSServerAddresses := helpers.BoshInstances("test-http-dns-server")
	if len(testHTTPDNSServerAddresses) == 0 {
		return ""
	}

	return helpers.BoshInstances("test-http-dns-server")[0].IP
}

func ensureRecursorIsDefinedByBoshAgent() {
	manifestPath := assetPath(testManifestName())
	disableOverridePath := assetPath(noRecursorsOpsFile())
	excludedRecursorsPath := assetPath(excludedRecursorsOpsFile())

	updateCloudConfigWithOurLocalRecursor()

	helpers.Bosh(
		"deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-o", disableOverridePath,
		"-o", excludedRecursorsPath,
		"--vars-store", "creds.yml",
		manifestPath,
	)
	allDeployedInstances = helpers.BoshInstances("bosh-dns")
}

func ensureRecursorIsDefinedByDNSRelease() {
	manifestPath := assetPath(testManifestName())
	configureRecursorPath := assetPath(configureRecursorOpsFile())

	updateCloudConfigWithDefaultCloudConfig()

	helpers.Bosh(
		"deploy",
		"-o", configureRecursorPath,
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-v", fmt.Sprintf("recursor_a=%s", RecursorIPAddresses[0]),
		"-v", fmt.Sprintf("recursor_b=%s", RecursorIPAddresses[1]),
		"--vars-store", "creds.yml",
		manifestPath,
	)
	allDeployedInstances = helpers.BoshInstances("bosh-dns")
}

func updateCloudConfigWithOurLocalRecursor() {
	removeRecursorAddressesOpsFile := assetPath(setupLocalRecursorOpsFile())

	helpers.Bosh(
		"update-cloud-config",
		"-o", removeRecursorAddressesOpsFile,
		"-v", "network=director_network",
		"-v", fmt.Sprintf("recursor_a=%s", RecursorIPAddresses[0]),
		"-v", fmt.Sprintf("recursor_b=%s", RecursorIPAddresses[1]),
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

func ensureHealthEndpointDeployed(extraOps ...string) {
	manifestPath := assetPath(testManifestName())

	updateCloudConfigWithDefaultCloudConfig()

	args := []string{
		"deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
		"-v", "health_server_port=2345",
		"-o", assetPath(enableHealthManifestOps()),
		"--vars-store", "creds.yml",
		manifestPath,
	}

	args = append(args, extraOps...)
	helpers.Bosh(args...)

	allDeployedInstances = helpers.BoshInstances("bosh-dns")
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

	logger := logger.NewAsyncWriterLogger(logger.LevelDebug, io.Discard)
	client, err := tlsclient.New("health.bosh-dns", []byte(caCert), cert, 5*time.Second, logger)
	Expect(err).NotTo(HaveOccurred())
	return client
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

	data, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	var respJson healthResponse
	err = json.Unmarshal(data, &respJson)
	Expect(err).ToNot(HaveOccurred())

	return respJson
}

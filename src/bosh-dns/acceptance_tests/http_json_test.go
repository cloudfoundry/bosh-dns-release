package acceptance

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/gomegadns"
)

var _ = Describe("HTTP JSON Server integration", func() {
	var (
		firstInstance helpers.InstanceInfo
	)

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			manifestPath := assetPath(testManifestName())
			enableHTTPJSONEndpointsPath := assetPath(enableHTTPJSONEndpointsOpsFile())
			enableConfiguresHandler := assetPath("ops/manifest/enable-configures-handler-job.yml")
			configureRecursorPath := assetPath(configureRecursorOpsFile())

			updateCloudConfigWithDefaultCloudConfig()

			testHTTPDNSServerAddress := fmt.Sprintf(
				"http://%s:8081",
				testHTTPDNSServerIPAddress(),
			)
			helpers.Bosh(
				"deploy",
				"-o", configureRecursorPath,
				"-o", enableHTTPJSONEndpointsPath,
				"-o", enableConfiguresHandler,
				"-v", fmt.Sprintf("name=%s", boshDeployment),
				"-v", fmt.Sprintf("base_stemcell=%s", baseStemcell),
				"-v", fmt.Sprintf("http_json_server_address=%s", testHTTPDNSServerAddress),
				"-v", fmt.Sprintf("recursor_a=%s", RecursorIPAddresses[0]),
				"-v", fmt.Sprintf("recursor_b=%s", RecursorIPAddresses[1]),
				"--vars-store", "creds.yml",
				manifestPath,
			)

			allDeployedInstances = helpers.BoshInstances("bosh-dns")
			firstInstance = allDeployedInstances[0]
		})

		It("answers queries with the response from the http server", func() {
			dnsResponse := helpers.Dig("app-id.internal.local.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "192.168.0.1", "ttl": 0}),
			))
		})

		It("configures Bosh DNS handlers by producing a dns handlers json file", func() {
			dnsResponse := helpers.Dig("handler.internal.local.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.168.0.1", "ttl": 0}),
			))
		})
	})
})

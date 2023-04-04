package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	gomegadns "bosh-dns/gomega-dns"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("recursor", func() {
	var (
		firstBoshDNS helpers.InstanceInfo
	)

	// As we were moving acceptance tests into integration tests, the test below
	// is cannot be moved as it modifies /etc/resolv.conf. We need to figure out
	// how to make this change safely in integration_tests/*.
	// [#165801868]
	Context("when the recursors must be read from the system resolver list", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByBoshAgent()
			firstBoshDNS = allDeployedInstances[0]
		})

		AfterEach(func() {
			// put the old cloud config back to avoid other tests using this recursor by accident
			updateCloudConfigWithDefaultCloudConfig()
		})

		It("forwards queries to the configured recursors on port 53", func() {
			dnsResponse := helpers.Dig("example.com.", firstBoshDNS.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.10.10.10", "ttl": 5}),
			))
		})
	})
})

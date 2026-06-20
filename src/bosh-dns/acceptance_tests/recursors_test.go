package acceptance

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/gomegadns"
)

var _ = Describe("recursor", func() {
	var (
		firstBoshDNS helpers.InstanceInfo
	)

	// As we were moving acceptance tests into integration tests, the test below
	// is cannot be moved as it modifies /etc/resolv.conf. We need to figure out
	// how to make this change safely in integration_tests/*.
	// [#165801868]
	//
	// TODO: remove when Jammy goes EOL. On Noble, bosh-dns is only a handler for
	// BOSH-specific domains; systemd-resolved handles external queries directly so
	// bosh-dns never does recursion in production.
	Context("when the recursors must be read from the system resolver list", func() {
		BeforeEach(func() {
			if !helpers.OverrideNameserverFor(baseStemcell) {
				Skip("bosh-dns is not the system resolver, no need to test recursion")
			}
			ensureRecursorIsDefinedByBoshAgent()
			firstBoshDNS = allDeployedInstances[0]
		})

		AfterEach(func() {
			// put the old cloud config back to avoid other tests using this recursor by accident
			updateCloudConfigWithDefaultCloudConfig()
		})

		It("forwards queries to the configured recursors on port 53", func() {
			dnsResponse := helpers.RemoteDig(firstBoshDNS.Slug(), "example.com.")

			// Per RFC 2181 §5.4.1, the AA bit should only be set when the responding server is
			// itself authoritative for the zone — a forwarder shouldn't propagate it. bosh-dns
			// passing through aa from an upstream is technically non-conformant.
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.10.10.10", "ttl": 5}),
			))
		})
	})
})

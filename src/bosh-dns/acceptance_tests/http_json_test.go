package acceptance

import (
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
			ensureHTTPJSONEndpointDeployed()
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

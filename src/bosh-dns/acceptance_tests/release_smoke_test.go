package acceptance

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/gomegadns"
)

var _ = Describe("Integration", func() {
	var (
		firstInstance helpers.InstanceInfo
	)

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDNSRelease()
			firstInstance = allDeployedInstances[0]
		})

		It("resolves alias globs", func() {
			for _, alias := range []string{"asterisk.alias.", "another.asterisk.alias.", "yetanother.asterisk.alias."} {
				dnsResponse := helpers.RemoteDig(firstInstance.Slug(), alias)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
					gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
				))
			}
		})

		It("resolves aliases from links", func() {
			dnsResponse := helpers.RemoteDig(firstInstance.Slug(), "dns-acceptance-alias.bosh.")

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))

			dnsResponse = helpers.RemoteDig(firstInstance.Slug(), fmt.Sprintf("%s.placeholder-alias.bosh.", allDeployedInstances[0].InstanceID))

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
			))
		})
	})

	Context("Instance health", func() {
		var (
			osSuffix string
		)

		BeforeEach(func() {
			osSuffix = ""
			if testTargetOS == "windows" {
				osSuffix = "-windows"
			}
			ensureHealthEndpointDeployed("-o", assetPath("ops/manifest/enable-stop-a-job"+osSuffix+".yml"))
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			helpers.Bosh("start")

			Eventually(func() int {
				dnsResponse := helpers.RemoteDig(firstInstance.Slug(), "q-s0.bosh-dns.default.bosh-dns.bosh.")
				return len(dnsResponse.Answer)
			}, 3*time.Minute, 5*time.Second).Should(Equal(len(allDeployedInstances)))
		})

		It("returns a healthy response when the instance is running", func() {
			client := setupSecureGet()

			Eventually(func() string {
				return secureGetRespBody(client, firstInstance.IP, 2345).State
			}, 31*time.Second, 1*time.Second).Should(Equal("running"))
		})
	})
})

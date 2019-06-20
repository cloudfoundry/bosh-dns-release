package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	gomegadns "bosh-dns/gomega-dns"
	"fmt"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"
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
				dnsResponse := helpers.Dig(alias, firstInstance.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
					gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
				))
			}
		})

		It("resolves aliases from links", func() {
			dnsResponse := helpers.Dig("dns-acceptance-alias.bosh.", firstInstance.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))

			dnsResponse = helpers.Dig(fmt.Sprintf("%s.placeholder-alias.bosh.", allDeployedInstances[0].InstanceID), firstInstance.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
			))
		})

		It("should resolve specified upcheck", func() {
			dnsResponse := helpers.Dig("upcheck.bosh-dns.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "127.0.0.1", "ttl": 0}),
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
			Eventually(func() []dns.RR {
				dnsResponse := helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP)
				return dnsResponse.Answer
			}, 60*time.Second, 1*time.Second).Should(HaveLen(len(allDeployedInstances)))
		})

		It("returns a healthy response when the instance is running", func() {
			client := setupSecureGet()

			Eventually(func() string {
				return secureGetRespBody(client, firstInstance.IP, 2345).State
			}, 31*time.Second).Should(Equal("running"))
		})
	})
})

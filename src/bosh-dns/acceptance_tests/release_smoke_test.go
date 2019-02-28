package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	gomegadns "bosh-dns/gomega-dns"
	"fmt"
	"regexp"
	"strings"

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
		var (
			testRecursorAddress string
		)

		BeforeEach(func() {
			testRecursorAddress = testRecursorIPAddress()
			ensureRecursorIsDefinedByDNSRelease(testRecursorAddress)
			firstInstance = allDeployedInstances[0]
		})

		It("returns records for bosh instances", func() {
			dnsResponse := helpers.Dig(fmt.Sprintf("%s.bosh-dns.default.bosh-dns.bosh.", firstInstance.InstanceID), firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ContainElement(
				gomegadns.MatchResponse(gomegadns.Response{"ip": firstInstance.IP, "ttl": 0}),
			))
		})

		It("returns Rcode failure for arpaing bosh instances", func() {
			dnsResponse := helpers.ReverseDigWithOptions(firstInstance.IP, firstInstance.IP, helpers.DigOpts{SkipRcodeCheck: true})
			Expect(dnsResponse.Rcode).To(Equal(dns.RcodeServerFailure))
		})

		It("returns records for bosh instances found with query for all records", func() {
			Expect(allDeployedInstances).To(HaveLen(2))

			dnsResponse := helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))
		})

		It("returns records for bosh instances found with query for index", func() {
			Expect(allDeployedInstances).To(HaveLen(2))

			dnsResponse := helpers.Dig(fmt.Sprintf("q-i%s.bosh-dns.default.bosh-dns.bosh.", firstInstance.Index), firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": firstInstance.IP, "ttl": 0}),
			))
		})

		It("finds and resolves aliases specified in other jobs on the same instance", func() {
			Expect(allDeployedInstances).To(HaveLen(2))
			dnsResponse := helpers.Dig("internal.alias.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))
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

		It("resolves link provider aliases", func() {
			dnsResponse := helpers.Dig("dns-acceptance-alias.bosh.", firstInstance.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
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
			ensureHealthEndpointDeployed(testRecursorAddress, "-o", assetPath("ops/enable-stop-a-job"+osSuffix+".yml"))
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

		It("stops returning IP addresses of instances whose status becomes unknown", func() {
			Expect(allDeployedInstances).To(HaveLen(2))

			dnsResponse := helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))

			secondInstanceSlug := fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID)
			helpers.Bosh("stop", secondInstanceSlug)

			Eventually(func() []dns.RR {
				return helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP).Answer
			}, 60*time.Second, 1*time.Second).Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": firstInstance.IP}),
			))
		})

		It("stops returning IP addresses of instances that become unhealthy", func() {
			Expect(allDeployedInstances).To(HaveLen(2))

			dnsResponse := helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[0].IP, "ttl": 0}),
				gomegadns.MatchResponse(gomegadns.Response{"ip": allDeployedInstances[1].IP, "ttl": 0}),
			))

			instanceSlug := fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID)
			helpers.BoshRunErrand("stop-a-job"+osSuffix, instanceSlug)

			Eventually(func() []dns.RR {
				return helpers.Dig("q-s0.bosh-dns.default.bosh-dns.bosh.", firstInstance.IP).Answer
			}, 60*time.Second, 1*time.Second).Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": firstInstance.IP}),
			))
		})

		Context("when a job defines a healthy executable", func() {
			var (
				osSuffix string
			)

			BeforeEach(func() {
				osSuffix = ""
				if testTargetOS == "windows" {
					osSuffix = "-windows"
				}
				ensureHealthEndpointDeployed(testRecursorAddress, "-o", assetPath("ops/enable-healthy-executable-job"+osSuffix+".yml"))
			})

			It("changes the health endpoint return value based on how the executable exits", func() {
				client := setupSecureGet()
				lastInstance := allDeployedInstances[1]
				lastInstanceSlug := fmt.Sprintf("%s/%s", lastInstance.InstanceGroup, lastInstance.InstanceID)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("running"))

				helpers.BoshRunErrand("make-health-executable-job-unhealthy"+osSuffix, lastInstanceSlug)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("failing"))

				helpers.BoshRunErrand("make-health-executable-job-healthy"+osSuffix, lastInstanceSlug)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("running"))
			})
		})
	})

	Describe("link dns names", func() {
		var (
			osSuffix string
		)

		BeforeEach(func() {
			osSuffix = ""
			if testTargetOS == "windows" {
				osSuffix = "-windows"
			}
			ensureHealthEndpointDeployed(
				testRecursorAddress,
				"-o", assetPath("ops/enable-link-dns-addresses.yml"),
				"-o", assetPath("ops/enable-healthy-executable-job"+osSuffix+".yml"),
			)
			firstInstance = allDeployedInstances[0]
		})

		It("respects health status according to job providing link", func() {
			client := setupSecureGet()
			lastInstance := allDeployedInstances[1]
			lastInstanceSlug := fmt.Sprintf("%s/%s", lastInstance.InstanceGroup, lastInstance.InstanceID)
			output := helpers.BoshRunErrand("get-healthy-executable-linked-address"+osSuffix, lastInstanceSlug)
			address := strings.TrimSpace(strings.Split(strings.Split(output, "ADDRESS:")[1], "\n")[0])
			Expect(address).To(MatchRegexp(`^q-n\d+s0\.q-g\d+\.bosh$`))
			re := regexp.MustCompile(`^q-n\d+s0\.q-g(\d+)\.bosh$`)
			groupID := re.FindStringSubmatch(address)[1]

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("running"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP, lastInstance.IP))

			helpers.BoshRunErrand("make-health-executable-job-unhealthy"+osSuffix, lastInstanceSlug)

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("failing"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP))

			helpers.BoshRunErrand("make-health-executable-job-healthy"+osSuffix, lastInstanceSlug)

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("running"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP, lastInstance.IP))
		})
	})
})

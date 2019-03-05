package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	gomegadns "bosh-dns/gomega-dns"
	"net"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const testRecursorAddress = "127.0.0.1"

var _ = Describe("recursor", func() {
	var (
		firstInstance       helpers.InstanceInfo
		testRecursorAddress string
	)

	Context("when the recursors must be read from the system resolver list", func() {
		BeforeEach(func() {
			testRecursorAddress = testRecursorIPAddress()
			ensureRecursorIsDefinedByBoshAgent(testRecursorAddress)
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			// put the old cloud config back to avoid other tests using this recursor by accident
			updateCloudConfigWithDefaultCloudConfig()
		})

		It("fowards queries to the configured recursors on port 53", func() {
			dnsResponse := helpers.Dig("example.com.", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.10.10.10", "ttl": 5}),
			))
		})
	})

	Context("when the recursors are configured explicitly on the DNS server", func() {
		BeforeEach(func() {
			testRecursorAddress = testRecursorIPAddress()
			ensureRecursorIsDefinedByDNSRelease(testRecursorAddress)
			firstInstance = allDeployedInstances[0]
		})

		It("returns success when receiving a truncated responses from a recursor", func() {
			By("ensuring the test recursor is returning truncated messages", func() {
				dnsResponse := helpers.Dig("truncated-recursor.com.", testRecursorAddress)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})

			By("ensuring the dns release returns a successful truncated recursed answer", func() {
				dnsResponse := helpers.Dig("truncated-recursor.com.", firstInstance.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})
		})

		It("timeouts when recursor takes longer than configured recursor_timeout", func() {
			By("ensuring the test recursor is working", func() {
				dnsResponse := helpers.DigWithOptions("slow-recursor.com.", testRecursorAddress, helpers.DigOpts{Timeout: 10 * time.Second})

				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})

			By("ensuring the dns release returns a error due to recursor timing out", func() {
				dnsResponse := helpers.DigWithOptions("slow-recursor.com.", firstInstance.IP, helpers.DigOpts{SkipRcodeCheck: true, Timeout: 5 * time.Second})
				Expect(dnsResponse.Rcode).To(Equal(dns.RcodeServerFailure))
			})
		})

		It("forwards large UDP EDNS messages", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("udp-9k-message.com.", testRecursorAddress, helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(270))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				dnsResponse := helpers.DigWithOptions("udp-9k-message.com.", firstInstance.IP, helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(270))
			})
		})

		It("compresses message responses that are larger than requested UDPSize", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("compressed-ip-truncated-recursor-large.com.", testRecursorAddress, helpers.DigOpts{BufferSize: 16384})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(512))
			})

			By("ensuring the dns release returns a successful compressed recursed answer", func() {
				dnsResponse := helpers.DigWithOptions("compressed-ip-truncated-recursor-large.com.", firstInstance.IP, helpers.DigOpts{BufferSize: 16384})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(512))
			})
		})

		It("forwards large dns answers even if udp response size is larger than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("ip-truncated-recursor-large.com.", testRecursorAddress, helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(20))
			})

			By("ensuring the dns release returns a successful truncated recursed answer", func() {
				dnsResponse := helpers.Dig("ip-truncated-recursor-large.com.", firstInstance.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(20))
			})
		})

		It("does not bother to compress messages that are smaller than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("recursor-small.com.", testRecursorAddress, helpers.DigOpts{BufferSize: 1})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(2))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				dnsResponse := helpers.Dig("recursor-small.com.", firstInstance.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(2))
			})
		})

		It("fowards queries to the configured recursors", func() {
			dnsResponse := helpers.Dig("example.com.", firstInstance.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.10.10.10", "ttl": 5}),
			))
		})

		It("forwards ipv4 ARPA queries to the configured recursors", func() {
			dnsResponse := helpers.ReverseDig("8.8.4.4", firstInstance.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"name": "4.4.8.8.in-addr.arpa."}),
			))
		})

		It("fowards ipv6 ARPA queries to the configured recursors", func() {
			dnsResponse := helpers.IPv6ReverseDig("2001:4860:4860::8888", firstInstance.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"name": "8.8.8.8.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.ip6.arpa."}),
			))
		})
	})

	Context("when using cache", func() {
		BeforeEach(func() {
			testRecursorAddress = testRecursorIPAddress()
			ensureRecursorIsDefinedByDNSRelease(testRecursorAddress)
			firstInstance = allDeployedInstances[0]
		})

		It("caches dns recursed dns entries for the duration of the TTL", func() {
			dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstInstance.IP)

			Expect(dnsResponse.Answer).To(HaveLen(1))
			dnsAnswer := dnsResponse.Answer[0]

			initialIP := gomegadns.FetchIP(dnsAnswer)

			Expect(dnsAnswer).To(gomegadns.MatchResponse(gomegadns.Response{
				"ttl": 5,
				"ip":  initialIP,
			}))

			Consistently(func() []dns.RR {
				dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstInstance.IP)
				return dnsResponse.Answer
			}, "4s", "1s").Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{
					"ip": initialIP,
				}),
			))

			nextIP := net.ParseIP(initialIP).To4()
			nextIP[3]++

			Eventually(func() []dns.RR {
				dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstInstance.IP)
				return dnsResponse.Answer
			}, "4s", "1s").Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{
					"ip": nextIP.String(),
				}),
			))
		})
	})
})

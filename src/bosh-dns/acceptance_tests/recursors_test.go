package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/dns/server/handlers"
	gomegadns "bosh-dns/gomega-dns"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("recursor", func() {
	var (
		firstBoshDNS helpers.InstanceInfo
	)

	Context("when the upstream recursor response differs", func() {
		const (
			testQuestion = "question_with_configurable_response."
		)

		BeforeEach(func() {
			deployTestRecursors()
		})

		Context("recursor selection", func() {
			Context("serial", func() {
				BeforeEach(func() {
					ensureRecursorSelectionIsSerial()
					firstBoshDNS = allDeployedInstances[0]
				})

				It("chooses recursors serially", func() {
					Consistently(func() dns.RR {
						helpers.Bosh(
							"-d",
							"bosh-dns",
							"restart",
						)

						dnsResponse := helpers.Dig(testQuestion, firstBoshDNS.IP)
						Expect(dnsResponse.Answer).To(HaveLen(1))

						return dnsResponse.Answer[0]
					}, 2*time.Minute).Should(gomegadns.MatchResponse(gomegadns.Response{"ip": "1.1.1.1"}))
				})
			})

			Context("smart", func() {
				BeforeEach(func() {
					ensureRecursorSelectionIsSmart()
					firstBoshDNS = allDeployedInstances[0]
				})

				It("shuffles recursors", func() {
					dnsResponse := helpers.Dig(testQuestion, firstBoshDNS.IP)
					Expect(dnsResponse.Answer).To(HaveLen(1))

					initialUpstreamResponse := dnsResponse.Answer[0]

					Eventually(func() dns.RR {
						helpers.Bosh(
							"-d",
							"bosh-dns",
							"restart",
						)

						dnsResponse := helpers.Dig(testQuestion, firstBoshDNS.IP)
						Expect(dnsResponse.Answer).To(HaveLen(1))

						return dnsResponse.Answer[0]
					}, 5*time.Minute).ShouldNot(Equal(initialUpstreamResponse))
				})
			})
		})

		Context("shifting recursor preference", func() {
			BeforeEach(func() {
				ensureRecursorSelectionIsSerial()
				firstBoshDNS = allDeployedInstances[0]
			})

			It("shifts", func() {
				By("verifying the recursors are healthy", func() {
					for _, ip := range RecursorIPAddresses {
						dnsResponse := helpers.Dig(testQuestion, ip)
						Expect(dnsResponse.Answer).To(HaveLen(1))
					}
				})

				By("querying the first upstream recursor", func() {
					dnsResponse := helpers.Dig(testQuestion, firstBoshDNS.IP)
					Expect(dnsResponse.Answer).To(ConsistOf(gomegadns.MatchResponse(gomegadns.Response{"ip": "1.1.1.1"})))
				})

				By("killing the first upstream recursor", func() {
					helpers.Bosh(
						"-d",
						"test-recursor",
						"stop",
						"test-recursor/0",
					)

					Eventually(func() string {
						return helpers.BoshInstances("test-recursor")[0].ProcessState
					}).Should(Equal("stopped"))
				})

				By("verifying the second recursor is healthy", func() {
					dnsResponse := helpers.Dig(testQuestion, RecursorIPAddresses[1])
					Expect(dnsResponse.Answer).To(HaveLen(1))
				})

				By("forcing the preference shift to the second upstream recursor", func() {
					for i := 0; i < handlers.FailHistoryThreshold; i++ {
						time.Sleep(6 * time.Second)
						fmt.Printf("Running %d times out of %d total\n", i, handlers.FailHistoryThreshold)
						dnsResponse := helpers.Dig(
							testQuestion,
							firstBoshDNS.IP,
						)
						Expect(dnsResponse.Answer).To(ConsistOf(gomegadns.MatchResponse(gomegadns.Response{"ip": "2.2.2.2"})))
					}
				})

				By("bringing back the first upstream recursor", func() {
					helpers.Bosh(
						"-d",
						"test-recursor",
						"start",
						"test-recursor/0",
					)

					Eventually(func() string {
						return helpers.BoshInstances("test-recursor")[0].ProcessState
					}).Should(Equal("running"))
				})

				By("validating that we still use the second recursor", func() {
					Consistently(func() []dns.RR {
						return helpers.Dig(
							testQuestion,
							firstBoshDNS.IP,
						).Answer
					}, 2*time.Minute, 5*time.Second).Should(ConsistOf(gomegadns.MatchResponse(gomegadns.Response{"ip": "2.2.2.2"})))
				})
			})

			AfterEach(func() {
				helpers.Bosh(
					"-d",
					"test-recursor",
					"start",
					"test-recursor/0",
				)

				Eventually(func() bool {
					dnsResponse := helpers.Dig(testQuestion, RecursorIPAddresses[0])
					return len(dnsResponse.Answer) == 1
				}, 5*time.Second, 1*time.Second).Should(BeTrue())

				helpers.Bosh(
					"-d",
					"bosh-dns",
					"restart",
				)
			})
		})
	})

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

	Context("when the recursors are configured explicitly on the DNS server", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDNSRelease()
			firstBoshDNS = allDeployedInstances[0]
		})

		It("forwards queries to the configured recursors", func() {
			dnsResponse := helpers.Dig("example.com.", firstBoshDNS.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"ip": "10.10.10.10", "ttl": 5}),
			))
		})
	})

	Context("handling upstream recursor responses", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDNSRelease()
			firstBoshDNS = allDeployedInstances[0]
		})

		It("returns success when receiving a truncated responses from a recursor", func() {
			By("ensuring the test recursor is returning truncated messages", func() {
				dnsResponse := helpers.Dig("truncated-recursor.com.", RecursorIPAddresses[0])
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})

			By("ensuring the dns release returns a successful response when the recursor truncates the answer", func() {
				dnsResponse := helpers.Dig("truncated-recursor.com.", firstBoshDNS.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})
		})

		It("timeouts when recursor takes longer than configured recursor_timeout", func() {
			By("ensuring the test recursor is working", func() {
				dnsResponse := helpers.DigWithOptions("slow-recursor.com.", RecursorIPAddresses[0], helpers.DigOpts{Timeout: 10 * time.Second})

				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(1))
			})

			By("ensuring the dns release returns a error due to recursor timing out", func() {
				dnsResponse := helpers.DigWithOptions("slow-recursor.com.", firstBoshDNS.IP, helpers.DigOpts{SkipRcodeCheck: true, Timeout: 5 * time.Second})
				Expect(dnsResponse.Rcode).To(Equal(dns.RcodeServerFailure))
			})
		})

		It("forwards large UDP EDNS messages", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("udp-9k-message.com.", RecursorIPAddresses[0], helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(270))
			})

			By("ensuring the dns release returns a successful response from a truncated recursor answer", func() {
				dnsResponse := helpers.DigWithOptions("udp-9k-message.com.", firstBoshDNS.IP, helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(270))
			})
		})

		It("compresses message responses that are larger than requested UDP Size", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("compressed-ip-truncated-recursor-large.com.", RecursorIPAddresses[0], helpers.DigOpts{BufferSize: 16384})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(512))
			})

			By("ensuring the dns release returns a successful response when the recursor answer is compressed", func() {
				dnsResponse := helpers.DigWithOptions("compressed-ip-truncated-recursor-large.com.", firstBoshDNS.IP, helpers.DigOpts{BufferSize: 16384})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(512))
			})
		})

		It("forwards large dns answers even if udp response size is larger than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("ip-truncated-recursor-large.com.", RecursorIPAddresses[0], helpers.DigOpts{BufferSize: 65535})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(20))
			})

			By("ensuring the dns release returns a successful response when the recursor answer is truncated", func() {
				dnsResponse := helpers.Dig("ip-truncated-recursor-large.com.", firstBoshDNS.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "tc", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(20))
			})
		})

		It("does not bother to compress messages that are smaller than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				dnsResponse := helpers.DigWithOptions("recursor-small.com.", RecursorIPAddresses[0], helpers.DigOpts{BufferSize: 1})
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(2))
			})

			By("ensuring the dns release returns a successful response", func() {
				dnsResponse := helpers.Dig("recursor-small.com.", firstBoshDNS.IP)
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(HaveLen(2))
			})
		})

		It("forwards ipv4 ARPA queries to the configured recursors", func() {
			dnsResponse := helpers.ReverseDig("8.8.4.4", firstBoshDNS.IP)
			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"name": "4.4.8.8.in-addr.arpa."}),
			))
		})

		It("forwards ipv6 ARPA queries to the configured recursors", func() {
			dnsResponse := helpers.IPv6ReverseDig("2001:4860:4860::8888", firstBoshDNS.IP)

			Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
			Expect(dnsResponse.Answer).To(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{"name": "8.8.8.8.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.ip6.arpa."}),
			))
		})
	})

	Context("when using cache", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDNSRelease()
			firstBoshDNS = allDeployedInstances[0]
		})

		It("caches upstream dns entries for the duration of the TTL", func() {
			dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstBoshDNS.IP)

			Expect(dnsResponse.Answer).To(HaveLen(1))
			dnsAnswer := dnsResponse.Answer[0]

			initialIP := gomegadns.FetchIP(dnsAnswer)

			Expect(dnsAnswer).To(gomegadns.MatchResponse(gomegadns.Response{
				"ttl": 5,
				"ip":  initialIP,
			}))

			Consistently(func() []dns.RR {
				dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstBoshDNS.IP)
				return dnsResponse.Answer
			}, "4s", "1s").Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{
					"ip": initialIP,
				}),
			))

			nextIP := net.ParseIP(initialIP).To4()
			nextIP[3]++

			Eventually(func() []dns.RR {
				dnsResponse := helpers.Dig("always-different-with-timeout-example.com.", firstBoshDNS.IP)
				return dnsResponse.Answer
			}, "4s", "1s").Should(ConsistOf(
				gomegadns.MatchResponse(gomegadns.Response{
					"ip": nextIP.String(),
				}),
			))
		})
	})
})

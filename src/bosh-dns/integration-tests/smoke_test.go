package integration_tests

import (
	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/dns/server/record"
	gomegadns "bosh-dns/gomega-dns"
	"fmt"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration", func() {
	Describe("Smoke Tests", func() {
		var (
			responses                                      []record.Record
			hosts                                          []record.Host
			instanceID, agentID, instanceIP, instanceIndex string
			e                                              TestEnvironment
		)

		BeforeEach(func() {
			// Make up an instanceid for records.json
			instanceID = "abcdabcd"
			agentID = "cbdacbda"
			instanceIP = "234.234.234.234"
			instanceIndex = "0"
			responses = []record.Record{record.Record{
				AgentID:       agentID,
				ID:            instanceID,
				IP:            instanceIP,
				InstanceIndex: instanceIndex,
			}}
			hosts = []record.Host{
				{
					IP:   "234.234.234.234",
					FQDN: "abcdabcd.group.network.deployment.domain",
				},
			}
		})

		JustBeforeEach(func() {
			e = NewTestEnvironment(responses, hosts, []string{}, false, "serial", []string{}, false)
			if err := e.Start(); err != nil {
				Fail(fmt.Sprintf("could not start test environment: %s", err))
			}
		})

		AfterEach(func() {
			if err := e.Stop(); err != nil {
				Fail(fmt.Sprintf("Failed to stop bosh-dns test environment: %s", err))
			}
		})

		Context("DNS endpoint", func() {
			It("returns records for bosh instances", func() {
				dnsResponse := helpers.DigWithPort(
					fmt.Sprintf("%s.bosh-dns.default.bosh-dns.bosh.", instanceID),
					e.ServerAddress(), e.Port())
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ContainElement(
					gomegadns.MatchResponse(gomegadns.Response{"ip": responses[0].IP, "ttl": 0}),
				))
			})

			It("returns records for agent queries", func() {
				dnsResponse := helpers.DigWithPort(
					fmt.Sprintf("%s.bosh-agent-id.", agentID),
					e.ServerAddress(), e.Port())
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ContainElement(
					gomegadns.MatchResponse(gomegadns.Response{"ip": responses[0].IP, "ttl": 0}),
				))
			})

			It("returns PTR for arpaing bosh instances", func() {
				dnsResponse := helpers.ReverseDigWithOptions(
					instanceIP,
					e.ServerAddress(),
					helpers.DigOpts{SkipRcodeCheck: true, Port: e.Port()},
				)
				Expect(dnsResponse.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(dnsResponse.Answer[0].(*dns.PTR).Ptr).To(Equal("abcdabcd.group.network.deployment.domain."))
			})

			It("returns name error for arpaing non-existent IPs", func() {
				dnsResponse := helpers.ReverseDigWithOptions(
					"192.168.1.1",
					e.ServerAddress(),
					helpers.DigOpts{SkipRcodeCheck: true, Port: e.Port()},
				)
				Expect(dnsResponse.Rcode).To(Equal(dns.RcodeNameError))
			})

			It("finds and resolves aliases specified in other jobs on the same instance", func() {
				dnsResponse := helpers.DigWithPort("internal.alias.", e.ServerAddress(), e.Port())
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{"ip": responses[0].IP, "ttl": 0}),
				))
			})

			It("should resolve specified upcheck", func() {
				dnsResponse := helpers.DigWithPort("internal.alias.", e.ServerAddress(), e.Port())
				Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
				Expect(dnsResponse.Answer).To(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{"ip": responses[0].IP, "ttl": 0}),
				))
			})

			Context("when a query has multiple records", func() {
				var (
					instanceIP2 string
				)

				BeforeEach(func() {
					instanceIP2 = "235.235.235.235"
					responses = append(responses, record.Record{
						ID:            "bcdabcda",
						IP:            instanceIP2,
						InstanceIndex: "2",
					})
				})

				It("returns all records for the s0 query", func() {
					dnsResponse := helpers.DigWithPort("q-s0.bosh-dns.default.bosh-dns.bosh.", e.ServerAddress(), e.Port())
					Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
					Expect(dnsResponse.Answer).To(ConsistOf(
						gomegadns.MatchResponse(gomegadns.Response{"ip": responses[0].IP, "ttl": 0}),
						gomegadns.MatchResponse(gomegadns.Response{"ip": responses[1].IP, "ttl": 0}),
					))
				})

				It("returns records for bosh instances found with query for index", func() {
					dnsResponse := helpers.DigWithPort(
						fmt.Sprintf("q-i%s.bosh-dns.default.bosh-dns.bosh.", "2"),
						e.ServerAddress(), e.Port())
					Expect(dnsResponse).To(gomegadns.HaveFlags("qr", "aa", "rd", "ra"))
					Expect(dnsResponse.Answer).To(ConsistOf(
						gomegadns.MatchResponse(gomegadns.Response{"ip": responses[1].IP, "ttl": 0}),
					))
				})
			})
		})
	})
})

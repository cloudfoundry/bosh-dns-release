package integration_tests

import (
	"bosh-dns/acceptance_tests/helpers"
	"bosh-dns/dns/server/record"
	gomegadns "bosh-dns/gomega-dns"
	"bosh-dns/healthcheck/api"
	"fmt"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration", func() {
	Describe("Health Smoke Tests", func() {
		var (
			t *testHealthServer
		)

		BeforeEach(func() {
			var err error
			t = NewTestHealthServer("127.0.0.2")
			err = t.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			t.Stop()
		})

		Context("when a job defines a healthy executable", func() {
			It("changes the health endpoint return value based on how the executable exits", func() {
				err := t.MakeHealthyExit(0, 1)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() api.HealthStatus {
					r, err := t.GetResponseBody()
					Expect(err).ToNot(HaveOccurred())
					return r.State
				}, 31*time.Second).Should(Equal(api.StatusFailing))

				err = t.MakeHealthyExit(0, 0)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() api.HealthStatus {
					r, err := t.GetResponseBody()
					Expect(err).ToNot(HaveOccurred())
					return r.State
				}, 31*time.Second).Should(Equal(api.StatusRunning))
			})

			It("respects health status according to job providing link", func() {
				By("Making the first job unhealthy")
				err := t.MakeHealthyExit(0, 1)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() api.HealthStatus {
					r, err := t.GetResponseBody()
					Expect(err).NotTo(HaveOccurred())
					return r.GroupState["0"]
				}, 31*time.Second).Should(Equal(api.StatusFailing))

				state, err := t.GetResponseBody()
				Expect(err).NotTo(HaveOccurred())
				Expect(state.State).To(Equal(api.StatusFailing))
				Expect(state.GroupState["1"]).To(Equal(api.StatusRunning))

				By("Making the first job healthy again")
				err = t.MakeHealthyExit(0, 0)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() api.HealthStatus {
					r, err := t.GetResponseBody()
					Expect(err).NotTo(HaveOccurred())
					return r.GroupState["0"]
				}, 31*time.Second).Should(Equal(api.StatusRunning))
			})
		})

		Context("when doing health DNS queries", func() {
			var (
				responses []record.Record
				e         TestEnvironment
			)

			BeforeEach(func() {
				responses = []record.Record{record.Record{
					ID:            "uuid",
					IP:            "127.0.0.2",
					InstanceIndex: "0",
					GroupIDs:      []string{"0", "1"},
				},
					record.Record{
						ID:            "uuid-1",
						IP:            "127.0.0.3",
						InstanceIndex: "1",
						GroupIDs:      []string{"0"},
					}}
			})

			JustBeforeEach(func() {
				e = NewTestEnvironment(responses, []string{}, false, "serial", []string{}, true)
				if err := e.Start(); err != nil {
					Fail(fmt.Sprintf("could not start test environment: %s", err))
				}
			})

			AfterEach(func() {
				if err := e.Stop(); err != nil {
					Fail(fmt.Sprintf("Failed to stop bosh-dns test environment: %s", err))
				}
			})

			It("respects health status according to job providing link querying via DNS", func() {
				Eventually(func() []dns.RR {
					dnsResponse := helpers.DigWithOptions(
						fmt.Sprintf("q-g0s0.bosh-dns.default.bosh-dns.bosh."), e.ServerAddress(),
						helpers.DigOpts{Port: e.Port(), SkipRcodeCheck: true})
					return dnsResponse.Answer
				}, 10 *time.Second).Should(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{
						"ip":  "127.0.0.2",
						"ttl": 0,
					}),
				))

				By("Making the first job unhealthy")
				err := t.MakeHealthyExit(0, 1)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() []dns.RR {
					dnsResponse := helpers.DigWithPort(
						fmt.Sprintf("q-g0s0.bosh-dns.default.bosh-dns.bosh."),
						e.ServerAddress(), e.Port())
					return dnsResponse.Answer
				}, 31*time.Second).Should(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{
						"ip":  "127.0.0.2",
						"ttl": 0,
					}),
					gomegadns.MatchResponse(gomegadns.Response{
						"ip":  "127.0.0.3",
						"ttl": 0,
					}),
				))

				By("Making the first job healthy again")
				err = t.MakeHealthyExit(0, 0)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() []dns.RR {
					dnsResponse := helpers.DigWithPort(
						fmt.Sprintf("q-g0s0.bosh-dns.default.bosh-dns.bosh."),
						e.ServerAddress(), e.Port())
					return dnsResponse.Answer
				}, 31*time.Second).Should(ConsistOf(
					gomegadns.MatchResponse(gomegadns.Response{
						"ip":  "127.0.0.2",
						"ttl": 0,
					}),
				))
			})
		})
	})
})

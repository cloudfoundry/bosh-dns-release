package tracker_test

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/tracker"
	"bosh-dns/dns/server/tracker/fakes"

	logfake "github.com/cloudfoundry/bosh-utils/logger/fakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracker", func() {
	var (
		subscription   chan []record.Record
		healthMonitor  chan record.Host
		trackedDomains *fakes.LimitedTranscript
		shutdown       chan struct{}
		hw             *fakes.Healther
		qf             *fakes.Query
		fakeLogger     *logfake.FakeLogger
	)

	BeforeEach(func() {
		subscription = make(chan []record.Record, 5)
		healthMonitor = make(chan record.Host, 5)
		shutdown = make(chan struct{})
		trackedDomains = &fakes.LimitedTranscript{}
		fakeLogger = &logfake.FakeLogger{}
		hw = &fakes.Healther{}
		qf = &fakes.Query{}
	})

	AfterEach(func() {
		close(shutdown)
	})

	Describe("Start", func() {
		BeforeEach(func() {
			trackedDomains.RegistryReturns([]string{"qs-foo.now.remove.me", "qs-foo.dont.remove.me", "qs-foo.now.update.me"})

			qf.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
				crit := mm.(criteria.Criteria)
				fqdn := crit["fqdn"][0]

				result := []record.Record{}
				for _, r := range recs {
					if r.Domain == fqdn {
						result = append(result, r)
					}
				}
				return result
			}

			tracker.Start(shutdown, subscription, healthMonitor, trackedDomains, hw, qf, fakeLogger)
		})

		Context("when notified to monitor records", func() {
			It("monitors the records", func() {
				healthMonitor <- record.Host{FQDN: "qs-foo.doesnt.matter.anyway", IP: "8.8.8.8"}
				Eventually(hw.TrackCallCount).Should(Equal(1))
				Expect(hw.TrackArgsForCall(0)).To(Equal("8.8.8.8"))
			})
		})

		Context("when we exceed the transcript length", func() {
			It("it untracks old records", func() {
				trackedDomains.TouchReturns("qs-foo.doesnt.matter.anyway")
				healthMonitor <- record.Host{FQDN: "qs-foo.doesnt.matter.anyway", IP: "8.8.8.8"}
				healthMonitor <- record.Host{FQDN: "qs-bar.doesnt.matter.anyway", IP: "1.1.1.1"}

				Eventually(trackedDomains.TouchCallCount).Should(Equal(2))
				Expect(trackedDomains.TouchArgsForCall(0)).To(Equal("qs-foo.doesnt.matter.anyway"))
				Expect(trackedDomains.TouchArgsForCall(1)).To(Equal("qs-bar.doesnt.matter.anyway"))

				Eventually(hw.UntrackCallCount).Should(Equal(1))
				Expect(hw.UntrackArgsForCall(0)).To(Equal("8.8.8.8"))
				Eventually(hw.TrackCallCount).Should(Equal(2))
			})

			It("doesn't retrack untracked records when we get notified of a subscription", func() {
				trackedDomains.TouchReturns("qs-foo.doesnt.matter.anyway")
				healthMonitor <- record.Host{FQDN: "qs-foo.doesnt.matter.anyway", IP: "8.8.8.8"}
				Eventually(hw.TrackCallCount).Should(Equal(1))
				Expect(hw.TrackArgsForCall(0)).To(Equal("8.8.8.8"))

				Eventually(trackedDomains.TouchCallCount).Should(Equal(1))
				Expect(trackedDomains.TouchArgsForCall(0)).To(Equal("qs-foo.doesnt.matter.anyway"))

				trackedDomains.RegistryReturns([]string{"qs-foo.doesnt.matter.anyway"})
				subscription <- []record.Record{
					{IP: "1.1.1.1", Domain: "qs-foo.doesnt.matter.anyway"},
				}

				Eventually(hw.UntrackCallCount).Should(Equal(1))
				Expect(hw.UntrackArgsForCall(0)).To(Equal("8.8.8.8"))

				Eventually(hw.TrackCallCount).Should(Equal(2))
				Eventually(hw.TrackArgsForCall(1)).Should(Equal("1.1.1.1"))
			})
		})

		Context("when notified of a subscription", func() {
			var (
				initialRecords []record.Record
			)
			BeforeEach(func() {
				initialRecords = []record.Record{
					{IP: "1.1.1.1", Domain: "qs-foo.now.remove.me"},
					{IP: "2.2.2.2", Domain: "qs-foo.dont.remove.me"},
					{IP: "3.3.3.3", Domain: "qs-foo.now.donttrack.me"},
					{IP: "4.4.4.4", Domain: "qs-foo.now.update.me"},
				}
				subscription <- initialRecords

				Eventually(qf.FilterCallCount).Should(Equal(3))
			})

			Context("initial subscription", func() {
				It("tracks IPs for all monitored domains", func() {
					Eventually(hw.TrackCallCount).Should(Equal(3))
					Expect(hw.TrackArgsForCall(0)).To(Equal("1.1.1.1"))
					Expect(hw.TrackArgsForCall(1)).To(Equal("2.2.2.2"))
					Expect(hw.TrackArgsForCall(2)).To(Equal("4.4.4.4"))
					Expect(hw.UntrackCallCount()).To(Equal(0))
				})
			})

			Context("updated subscription", func() {
				var (
					updatedRecords []record.Record
				)
				BeforeEach(func() {
					trackedDomains.RegistryReturns([]string{"qs-foo.now.remove.me", "qs-foo.dont.remove.me", "qs-foo.now.update.me", "alias1.tld", "alias2.tld"})
					updatedRecords = []record.Record{
						// 1.1.1.1 removed
						{IP: "2.2.2.2", Domain: "qs-foo.dont.remove.me"},
						{IP: "3.3.3.3", Domain: "qs-foo.now.donttrack.me"},
						{IP: "5.5.5.5", Domain: "qs-foo.now.update.me"}, // 4.4.4.4 -> 5.5.5.5
						{IP: "6.6.6.6", Domain: "alias1.tld"},
						{IP: "6.6.6.6", Domain: "alias2.tld"}, // ip with multiple domains
					}
					subscription <- updatedRecords
				})

				Context("tracked domains have new IPs", func() {
					It("Gets tracked", func() {
						Eventually(hw.TrackCallCount).Should(Equal(5))
						Expect(hw.TrackArgsForCall(3)).To(Equal("5.5.5.5"))
						Expect(hw.TrackArgsForCall(4)).To(Equal("6.6.6.6"))
					})
				})
				Context("a tracked IP is no longer referenced by any tracked domain", func() {
					It("Gets untracked", func() {
						Eventually(hw.UntrackCallCount).Should(Equal(2))
						untrackedIps := []string{hw.UntrackArgsForCall(0), hw.UntrackArgsForCall(1)}

						Expect(untrackedIps).To(ConsistOf("1.1.1.1", "4.4.4.4")) // order not guaranteed
					})
				})
				Context("on subsequent subscriptions", func() {
					It("all previous state remembered", func() {
						Eventually(hw.TrackCallCount).Should(Equal(5))
						subscription <- updatedRecords
						Consistently(hw.TrackCallCount).Should(Equal(5))
						Consistently(hw.UntrackCallCount).Should(Equal(2))
					})
				})
			})
		})
	})
})

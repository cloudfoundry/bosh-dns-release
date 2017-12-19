package tracker_test

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/tracker"
	"bosh-dns/dns/server/tracker/fakes"

	. "github.com/onsi/ginkgo"
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
	)

	BeforeEach(func() {
		subscription = make(chan []record.Record, 5)
		healthMonitor = make(chan record.Host, 5)
		shutdown = make(chan struct{})
		trackedDomains = &fakes.LimitedTranscript{}
		hw = &fakes.Healther{}
		qf = &fakes.Query{}
	})

	AfterEach(func() {
		close(shutdown)
	})

	Describe("Start", func() {
		var nameIP map[string][]record.Record
		BeforeEach(func() {
			trackedDomains.RegistryReturns([]string{"qs-foo.now.remove.me", "qs-foo.dont.remove.me", "qs-foo.now.update.me"})

			nameIP = map[string][]record.Record{
				"qs-foo.now.remove.me":  []record.Record{{IP: "1.1.1.1", Domain: "qs-foo.now.remove.me"}},
				"qs-foo.dont.remove.me": []record.Record{{IP: "2.2.2.2", Domain: "qs-foo.dont.remove.me"}},
				"qs-foo.now.update.me":  []record.Record{{IP: "4.4.4.4", Domain: "qs-foo.dont.remove.me"}},
			}

			qf.FilterStub = func(crit criteria.Criteria, recs []record.Record) []record.Record {
				return nameIP[crit["fqdn"][0]]
			}

			tracker.Start(shutdown, subscription, healthMonitor, trackedDomains, hw, qf)
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
				nameIP["qs-foo.doesnt.matter.anyway"] = []record.Record{{IP: "1.1.1.1", Domain: "qs-foo.doesnt.matter.anyway"}}
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
			It("syncs the tracking behavior", func() {
				initialRecords := []record.Record{
					{IP: "1.1.1.1", Domain: "qs-foo.now.remove.me"},
					{IP: "2.2.2.2", Domain: "qs-foo.dont.remove.me"},
					{IP: "3.3.3.3", Domain: "qs-foo.now.donttrack.me"},
					{IP: "4.4.4.4", Domain: "qs-foo.now.update.me"},
				}
				subscription <- initialRecords
				Eventually(qf.FilterCallCount).Should(Equal(3))
				_, filterCallRecs := qf.FilterArgsForCall(0)
				Expect(filterCallRecs).To(Equal(initialRecords))

				Eventually(hw.TrackCallCount).Should(Equal(3))
				Expect(hw.TrackArgsForCall(0)).To(Equal("1.1.1.1"))
				Expect(hw.TrackArgsForCall(1)).To(Equal("2.2.2.2"))
				Expect(hw.TrackArgsForCall(2)).To(Equal("4.4.4.4"))
				nameIP["qs-foo.now.update.me"] = []record.Record{{IP: "5.5.5.5", Domain: "qs-foo.now.update"}}

				subscription <- []record.Record{
					{IP: "2.2.2.2", Domain: "qs-foo.dont.remove.me"},
					{IP: "3.3.3.3", Domain: "qs-foo.now.donttrack.me"},
					{IP: "5.5.5.5", Domain: "qs-foo.now.update.me"},
				}

				Eventually(hw.UntrackCallCount).Should(Equal(1))
				Expect(hw.UntrackArgsForCall(0)).To(Equal("4.4.4.4"))

				Eventually(hw.TrackCallCount).Should(Equal(4))
				Expect(hw.TrackArgsForCall(3)).To(Equal("5.5.5.5"))
			})
		})
	})
})

package records_test

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
	"bosh-dns/healthcheck/api"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type notRealCriteria struct{}

func (n *notRealCriteria) Matcher() criteria.Matcher {
	return &criteria.AndMatcher{}
}

var _ = Describe("HealthFilter", func() {
	var (
		fakeFilter        *recordsfakes.FakeReducer
		healthFilter      records.Reducer
		shouldTrack       bool
		healthChan        chan record.Host
		waitGroup         *sync.WaitGroup
		fakeHealthWatcher *healthinessfakes.FakeHealthWatcher
		clock             *fakeclock.FakeClock
		healthStrategy    string
		syncStrategy      string
		fqdn              string
		crit              criteria.MatchMaker
	)

	BeforeEach(func() {
		fakeFilter = &recordsfakes.FakeReducer{}
		waitGroup = &sync.WaitGroup{}
		clock = fakeclock.NewFakeClock(time.Now())
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		fqdn = "my-domain.some.fqdn.bosh."
		healthChan = make(chan record.Host, 2)

		fakeHealthWatcher.HealthStateStub = func(ip string) api.HealthResult {
			switch ip {
			case "1.1.1.1":
				return api.HealthResult{
					State: api.StatusRunning,
				}
			case "2.2.2.2":
				return api.HealthResult{
					State: api.StatusFailing,
				}
			case "3.3.3.3":
				return api.HealthResult{
					State: healthiness.StateUnknown,
				}
			case "4.4.4.4":
				return api.HealthResult{
					State: healthiness.StateUnchecked,
				}
			default:
				return api.HealthResult{}
			}
		}
	})

	JustBeforeEach(func() {
		hf := records.NewHealthFilter(fakeFilter, healthChan, fakeHealthWatcher, shouldTrack, clock, waitGroup)
		healthFilter = &hf
		crit = criteria.Criteria{
			"s":    []string{healthStrategy},
			"y":    []string{syncStrategy},
			"fqdn": []string{fqdn},
		}
	})

	Context("shouldTrack false", func() {
		BeforeEach(func() {
			shouldTrack = false
		})

		Context("default health strategy", func() {
			BeforeEach(func() {
				healthStrategy = ""
			})

			DescribeTable("it returns all records when all are in one category", func(rec record.Record) {
				fakeFilter.FilterReturns([]record.Record{rec})

				results := healthFilter.Filter(crit, []record.Record{rec})
				Expect(results).To(Equal([]record.Record{rec}))
			},
				Entry("healthy", record.Record{IP: "1.1.1.1"}),
				Entry("unhealthy", record.Record{IP: "2.2.2.2"}),
				Entry("unknown", record.Record{IP: "3.3.3.3"}),
				Entry("unchecked", record.Record{IP: "4.4.4.4"}),
			)

			DescribeTable("when one record is healthy and the others are", func(rec record.Record, included bool) {
				fakeFilter.FilterReturns([]record.Record{rec, record.Record{IP: "1.1.1.1"}})

				results := healthFilter.Filter(crit, []record.Record{rec})
				if included {
					Expect(results).To(ContainElement(rec))
				} else {
					Expect(results).ToNot(ContainElement(rec))
				}
				Expect(results).To(ContainElement(record.Record{IP: "1.1.1.1"}))
			},
				Entry("healthy", record.Record{IP: "1.1.1.1"}, true),
				Entry("unhealthy", record.Record{IP: "2.2.2.2"}, false),
				Entry("unknown", record.Record{IP: "3.3.3.3"}, false),
				Entry("unchecked", record.Record{IP: "4.4.4.4"}, true),
			)
		})
		Context("health strategy unhealthy only", func() {
			BeforeEach(func() {
				healthStrategy = "1"
			})

			DescribeTable("when one record is healthy and the others are", func(rec record.Record, included bool) {
				fakeFilter.FilterReturns([]record.Record{rec, record.Record{IP: "1.1.1.1"}})

				results := healthFilter.Filter(crit, []record.Record{rec})
				if included {
					Expect(results).To(ContainElement(rec))
				} else {
					Expect(results).ToNot(ContainElement(rec))
				}

				Expect(results).ToNot(ContainElement(record.Record{IP: "1.1.1.1"}))
			},
				Entry("healthy", record.Record{IP: "1.1.1.1"}, false),
				Entry("unhealthy", record.Record{IP: "2.2.2.2"}, true),
				Entry("unknown", record.Record{IP: "3.3.3.3"}, false),
				Entry("unchecked", record.Record{IP: "4.4.4.4"}, false),
			)
		})
		Context("health strategy healthy only", func() {
			BeforeEach(func() {
				healthStrategy = "3"
			})

			DescribeTable("when one record is healthy and the others are", func(rec record.Record, included bool) {
				fakeFilter.FilterReturns([]record.Record{rec, record.Record{IP: "1.1.1.1"}})

				results := healthFilter.Filter(crit, []record.Record{rec})
				if included {
					Expect(results).To(ContainElement(rec))
				} else {
					Expect(results).ToNot(ContainElement(rec))
				}

				Expect(results).To(ContainElement(record.Record{IP: "1.1.1.1"}))
			},
				Entry("healthy", record.Record{IP: "1.1.1.1"}, true),
				Entry("unhealthy", record.Record{IP: "2.2.2.2"}, false),
				Entry("unknown", record.Record{IP: "3.3.3.3"}, false),
				Entry("unchecked", record.Record{IP: "4.4.4.4"}, false),
			)
		})
		Context("health strategy all records", func() {
			BeforeEach(func() {
				healthStrategy = "4"
			})

			DescribeTable("when one record is healthy and the others are", func(rec record.Record, included bool) {
				fakeFilter.FilterReturns([]record.Record{rec, record.Record{IP: "1.1.1.1"}})

				results := healthFilter.Filter(crit, []record.Record{rec})
				if included {
					Expect(results).To(ContainElement(rec))
				} else {
					Expect(results).ToNot(ContainElement(rec))
				}

				Expect(results).To(ContainElement(record.Record{IP: "1.1.1.1"}))
			},
				Entry("healthy", record.Record{IP: "1.1.1.1"}, true),
				Entry("unhealthy", record.Record{IP: "2.2.2.2"}, true),
				Entry("unknown", record.Record{IP: "3.3.3.3"}, true),
				Entry("unchecked", record.Record{IP: "4.4.4.4"}, true),
			)
		})

		Context("link health querying", func() {
			Context("with healthy health-strategy", func() {

				BeforeEach(func() {
					healthStrategy = "3"
				})
				DescribeTable("when querying link healthiness", func(runningLinks, failingLinks, queriedLinks []string) {
					vmState := api.StatusRunning
					if len(failingLinks) > 0 {
						vmState = api.StatusFailing
					}
					result := api.HealthResult{
						State:      vmState,
						GroupState: map[string]api.HealthStatus{},
					}
					for _, runningLink := range runningLinks {
						result.GroupState[runningLink] = api.StatusRunning
					}
					for _, failingLink := range failingLinks {
						result.GroupState[failingLink] = api.StatusFailing
					}

					fakeHealthWatcher.HealthStateReturns(result)

					localCrit := crit.(criteria.Criteria)
					allQueriedAreRunning := true
					for _, queriedLink := range queriedLinks {
						localCrit["g"] = append(localCrit["g"], queriedLink)
						if allQueriedAreRunning {

							for _, failingLink := range failingLinks {
								if queriedLink == failingLink {
									allQueriedAreRunning = false
								}
							}
						}
					}
					crit = localCrit

					fakeFilter.FilterReturns([]record.Record{record.Record{IP: "1.1.1.1"}})
					results := healthFilter.Filter(crit, []record.Record{
						record.Record{IP: "1.1.1.1"},
					})

					if allQueriedAreRunning {
						Expect(results).To(ConsistOf([]record.Record{
							record.Record{IP: "1.1.1.1"},
						}))
					} else {
						Expect(results).To(BeEmpty())
					}
				},
					Entry("when there is a single queried and running link", []string{"1"}, []string{}, []string{"1"}),
					Entry("when there is a single queried and failing link", []string{}, []string{"1"}, []string{"1"}),
					Entry("when there are multiple queried links and all are running", []string{"1", "2"}, []string{}, []string{"1", "2"}),
					Entry("when there are multiple queried links and some are running", []string{"1"}, []string{"2"}, []string{"1", "2"}),
					Entry("when there are multiple queried links and some are running", []string{"1"}, []string{"2"}, []string{"1"}),
					Entry("when there are multiple queried links and some are running", []string{"1"}, []string{"2"}, []string{"2"}),
					Entry("when there are multiple queried links and all are failing", []string{}, []string{"1", "2"}, []string{"1", "2"}),
					Entry("when a queried link does not exist", []string{}, []string{}, []string{"1", "2"}),
				)
			})

			Context("with multiple records and default health strategy", func() {
				BeforeEach(func() {
					healthStrategy = "0"
				})

				It("honors link health", func() {
					localCrit := crit.(criteria.Criteria)
					localCrit["g"] = []string{"1", "2"}
					crit = localCrit

					recs := []record.Record{
						record.Record{
							IP: "1.1.1.1",
						},
						record.Record{
							IP: "2.2.2.2",
						},
						record.Record{
							IP: "3.3.3.3",
						},
					}

					fakeHealthWatcher.HealthStateStub = func(ip string) api.HealthResult {
						switch ip {
						case "1.1.1.1":
							return api.HealthResult{
								State: api.StatusRunning,
								GroupState: map[string]api.HealthStatus{
									"1": api.StatusRunning,
								},
							}
						case "2.2.2.2":
							return api.HealthResult{
								State: api.StatusRunning,
								GroupState: map[string]api.HealthStatus{
									"2": api.StatusFailing,
								},
							}
						case "3.3.3.3":
							return api.HealthResult{
								State: healthiness.StateUnchecked,
							}
						}
						return api.HealthResult{}
					}

					fakeFilter.FilterReturns(recs)
					results := healthFilter.Filter(crit, recs)
					Expect(results).To(ConsistOf([]record.Record{
						record.Record{IP: "1.1.1.1"},
						record.Record{IP: "3.3.3.3"},
					}))
				})
			})
		})
	})

	Context("shouldTrack true", func() {
		BeforeEach(func() {
			shouldTrack = true
		})

		Context("asynchronous", func() {
			BeforeEach(func() {
				syncStrategy = "0"
			})

			It("sends a message over the health channel for each record", func() {
				recs := []record.Record{
					record.Record{IP: "1.1.1.1"},
					record.Record{IP: "2.2.2.2"},
				}
				fakeFilter.FilterReturns(recs)
				healthFilter.Filter(crit, recs)

				Eventually(healthChan).Should(Receive(Equal(record.Host{IP: "1.1.1.1", FQDN: "my-domain.some.fqdn.bosh."})))
				Eventually(healthChan).Should(Receive(Equal(record.Host{IP: "2.2.2.2", FQDN: "my-domain.some.fqdn.bosh."})))
			})

			Context("when fqdn is not in criteria", func() {
				JustBeforeEach(func() {
					crit = criteria.Criteria{
						"s": []string{healthStrategy},
						"y": []string{syncStrategy},
					}
				})

				It("does not send a message", func() {
					recs := []record.Record{
						record.Record{IP: "1.1.1.1"},
						record.Record{IP: "2.2.2.2"},
					}
					fakeFilter.FilterReturns(recs)
					healthFilter.Filter(crit, recs)

					Consistently(healthChan).ShouldNot(Receive(Equal(record.Host{IP: "1.1.1.1", FQDN: "my-domain.some.fqdn.bosh."})))
					Consistently(healthChan).ShouldNot(Receive(Equal(record.Host{IP: "2.2.2.2", FQDN: "my-domain.some.fqdn.bosh."})))
				})
			})
		})

		Context("synchronous", func() {
			Context("syncStrategy 1", func() {
				var recs []record.Record
				BeforeEach(func() {
					syncStrategy = "1"

					fakeHealthWatcher.RunCheckStub = func(ip string) {
						fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: api.StatusRunning})
					}
					healthStrategy = "3"
					recs = []record.Record{
						record.Record{IP: "1.1.1.1"},
					}
					fakeFilter.FilterReturns(recs)
				})

				Context("when initial status is unchecked", func() {
					BeforeEach(func() {
						fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: healthiness.StateUnchecked})
					})

					It("will perform a healthcheck before returning results when the initial status is unchecked", func() {
						results := healthFilter.Filter(crit, recs)

						Expect(results).To(ConsistOf([]record.Record{
							record.Record{IP: "1.1.1.1"},
						}))
					})
				})

				Context("when the initial status is unknown", func() {
					BeforeEach(func() {
						fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: healthiness.StateUnknown})
					})

					It("won't perform a healthcheck before returning results", func() {
						results := healthFilter.Filter(crit, recs)

						Expect(results).To(BeEmpty())
					})
				})

				Context("when the initial status is unhealthy", func() {
					BeforeEach(func() {
						fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: api.StatusFailing})
					})

					It("won't perform a healthcheck before returning results", func() {
						results := healthFilter.Filter(crit, recs)

						Expect(results).To(BeEmpty())
					})
				})

				Context("when it takes too long to check health", func() {
					BeforeEach(func() {
						fakeHealthWatcher.RunCheckStub = func(ip string) {
							clock.Increment(2 * time.Second)
						}
						fakeHealthWatcher.HealthStateStub = func(ip string) api.HealthResult {
							return api.HealthResult{State: healthiness.StateUnchecked}
						}
					})

					It("times out", func() {
						waitGroup.Add(1)
						healthFilter.Filter(crit, recs)
						waitGroup.Done()
					})
				})
			})
		})
	})

	Context("without a real criteria", func() {
		var recs []record.Record
		BeforeEach(func() {
			recs = []record.Record{
				record.Record{IP: "2.2.2.2"},
				record.Record{IP: "3.3.3.3"},
			}
			fakeFilter.FilterReturns(recs)
		})

		JustBeforeEach(func() {
			crit = &notRealCriteria{}
		})

		It("returns all records based on the default health strategy", func() {
			results := healthFilter.Filter(crit, recs)
			Expect(results).To(ConsistOf(recs))
		})
	})
})

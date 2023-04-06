package healthiness_test

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/healthcheck/api"
)

var _ = Describe("HealthWatcher", func() {
	var (
		fakeChecker *healthinessfakes.FakeHealthChecker
		fakeClock   *fakeclock.FakeClock
		fakeLogger  *loggerfakes.FakeLogger
		interval    time.Duration
		signal      chan struct{}
		stopped     sync.WaitGroup

		healthWatcher healthiness.HealthWatcher
	)

	BeforeEach(func() {
		fakeChecker = &healthinessfakes.FakeHealthChecker{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeLogger = &loggerfakes.FakeLogger{}
		interval = time.Second
		healthWatcher = healthiness.NewHealthWatcher(1, fakeChecker, fakeClock, interval, fakeLogger)
		signal = make(chan struct{})
		stopped = sync.WaitGroup{}
		started := sync.WaitGroup{}

		started.Add(1)
		stopped.Add(1)

		go func() {
			started.Done()
			healthWatcher.Run(signal)
			stopped.Done()
		}()

		started.Wait()
	})

	AfterEach(func() {
		close(signal)
		stopped.Wait()
		Eventually(fakeClock.WatcherCount).Should(Equal(0))
	})

	Describe("HealthState", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.1"
		})

		Context("when the status is not known", func() {
			Context("and the ip is not known because it's not tracked", func() {
				It("returns unchecked", func() {
					Expect(healthWatcher.HealthState(ip).State).To(Equal(healthiness.StateUnchecked))
				})
			})

			Context("and the ip is known", func() {
				JustBeforeEach(func() {
					fakeChecker.GetStatusReturns(api.HealthResult{State: healthiness.StateUnknown})
					healthWatcher.Track(ip)
					Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))
					Expect(fakeChecker.GetStatusArgsForCall(0)).To(Equal(ip))
				})

				It("returns unknown", func() {
					Expect(healthWatcher.HealthState(ip).State).To(Equal(healthiness.StateUnknown))
				})
			})

			Context("and it takes a while to run a check", func() {
				JustBeforeEach(func() {
					fakeChecker.GetStatusStub = func(ip string) api.HealthResult {
						time.Sleep(1 * time.Second)
						return api.HealthResult{State: api.StatusFailing}
					}
				})

				It("other lookups will wait for the check", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(api.HealthResult{State: healthiness.StateUnchecked}))
					wg := sync.WaitGroup{}
					for i := 0; i < 5; i++ {
						wg.Add(1)
						go func() {
							result := healthWatcher.RunCheck(ip)
							Expect(result.State).To(Equal(api.StatusFailing))
							wg.Done()
						}()
					}
					wg.Wait()
					Expect(fakeChecker.GetStatusCallCount()).To(Equal(1))
					Expect(fakeChecker.GetStatusArgsForCall(0)).To(Equal(ip))
					Expect(healthWatcher.HealthState(ip).State).To(Equal(api.StatusFailing))
				})
			})
		})

		Context("when the status is known", func() {
			JustBeforeEach(func() {
				healthWatcher.Track(ip)
				Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))
				Expect(fakeChecker.GetStatusArgsForCall(0)).To(Equal(ip))
			})

			Context("and the ip is healthy", func() {
				BeforeEach(func() {
					ip = "127.0.0.2"
					fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusRunning})
				})

				It("returns healthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(api.HealthResult{State: api.StatusRunning}))
				})
			})

			Context("and the ip is unhealthy", func() {
				BeforeEach(func() {
					ip = "127.0.0.3"
					fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusFailing})
				})

				It("returns unhealthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(api.HealthResult{State: api.StatusFailing}))
				})
			})

			Context("and the status changes", func() {
				BeforeEach(func() {
					fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusRunning})
				})

				It("returns the new status", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(api.HealthResult{State: api.StatusRunning}))

					fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusFailing})
					fakeClock.WaitForWatcherAndIncrement(interval)

					Eventually(func() api.HealthResult {
						return healthWatcher.HealthState(ip)
					}).Should(Equal(api.HealthResult{State: api.StatusFailing}))
				})
			})

			Context("Redundant track request", func() {

				It("Does not send a redundant check", func() {
					healthWatcher.Track(ip)
					healthWatcher.Track(ip)

					Expect(fakeChecker.GetStatusCallCount()).To(Equal(1))
				})
			})
		})

		Context("tracking multiple ip addresses", func() {
			BeforeEach(func() {
				ip = "127.0.0.1"
				fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusRunning})

				// Track more IPs to exercise potential racey behaviors
				healthWatcher.Track("1.1.1.1")
				healthWatcher.Track("2.2.2.2")
				healthWatcher.Track("3.3.3.3")
			})

			JustBeforeEach(func() {
				healthWatcher.Track(ip)
				Eventually(fakeChecker.GetStatusCallCount).Should(Equal(4))
			})

			It("returns new statuses when they change", func() {
				Eventually(func() api.HealthStatus {
					return healthWatcher.HealthState(ip).State
				}).Should(Equal(api.StatusRunning))
				fakeChecker.GetStatusReturns(api.HealthResult{State: api.StatusFailing})
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(func() api.HealthStatus {
					return healthWatcher.HealthState(ip).State
				}).Should(Equal(api.StatusFailing))
			})
		})
	})

	Describe("Untrack", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.2"
			healthWatcher.Track(ip)
			Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))
			Expect(fakeChecker.GetStatusArgsForCall(0)).To(Equal(ip))
		})

		It("stops tracking the status", func() {
			healthWatcher.Untrack(ip)
			fakeClock.WaitForWatcherAndIncrement(interval)
			Consistently(fakeChecker.GetStatusCallCount).Should(Equal(1))
		})
	})
})

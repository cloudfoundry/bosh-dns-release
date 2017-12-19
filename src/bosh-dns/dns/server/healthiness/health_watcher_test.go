package healthiness_test

import (
	"time"

	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"

	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthWatcher", func() {
	var (
		fakeChecker *healthinessfakes.FakeHealthChecker
		fakeClock   *fakeclock.FakeClock
		interval    time.Duration
		signal      chan struct{}
		stopped     chan struct{}

		healthWatcher healthiness.HealthWatcher
	)

	BeforeEach(func() {
		fakeChecker = &healthinessfakes.FakeHealthChecker{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		interval = time.Second
		healthWatcher = healthiness.NewHealthWatcher(fakeChecker, fakeClock, interval)
		signal = make(chan struct{})
		stopped = make(chan struct{})

		go func() {
			healthWatcher.Run(signal)
			close(stopped)
		}()
	})

	AfterEach(func() {
		close(signal)
		Eventually(stopped).Should(BeClosed())
		Eventually(fakeClock.WatcherCount).Should(Equal(0))
	})

	Describe("IsHealthy", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.1"
		})

		Context("when the status is not known", func() {
			It("is always healthy", func() {
				Expect(healthWatcher.IsHealthy(ip)).To(BeTrue())
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
					fakeChecker.GetStatusReturns(true)
				})

				It("returns healthy", func() {
					Expect(healthWatcher.IsHealthy(ip)).To(BeTrue())
				})
			})

			Context("and the ip is unhealthy", func() {
				BeforeEach(func() {
					ip = "127.0.0.3"
					fakeChecker.GetStatusReturns(false)
				})

				It("returns unhealthy", func() {
					Expect(healthWatcher.IsHealthy(ip)).To(BeFalse())
				})
			})

			Context("and the status changes", func() {
				BeforeEach(func() {
					fakeChecker.GetStatusReturns(true)
				})

				It("goes unhealthy if the new status is stopped", func() {
					Expect(healthWatcher.IsHealthy(ip)).To(BeTrue())
					Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))

					fakeChecker.GetStatusReturns(false)

					Consistently(func() bool {
						return healthWatcher.IsHealthy(ip)
					}).Should(BeTrue())

					fakeClock.WaitForWatcherAndIncrement(interval)

					Eventually(func() bool {
						return healthWatcher.IsHealthy(ip)
					}).Should(BeFalse())
				})
			})
		})
	})

	Describe("HealthState", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.1"
		})

		Context("when the status is not known", func() {
			It("returns unknown", func() {
				Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateUnknown))
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
					fakeChecker.GetStatusReturns(true)
				})

				It("returns healthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateHealthy))
				})
			})

			Context("and the ip is unhealthy", func() {
				BeforeEach(func() {
					ip = "127.0.0.3"
					fakeChecker.GetStatusReturns(false)
				})

				It("returns unhealthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateUnhealthy))
				})
			})

			Context("and the status changes", func() {
				BeforeEach(func() {
					fakeChecker.GetStatusReturns(true)
				})

				It("goes unhealthy if the new status is stopped", func() {
					Expect(healthWatcher.IsHealthy(ip)).To(BeTrue())
					Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))

					fakeChecker.GetStatusReturns(false)

					Consistently(func() string {
						return healthWatcher.HealthState(ip)
					}).Should(Equal(healthiness.StateHealthy))

					fakeClock.WaitForWatcherAndIncrement(interval)

					Eventually(func() string {
						return healthWatcher.HealthState(ip)
					}).Should(Equal(healthiness.StateUnhealthy))
				})
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

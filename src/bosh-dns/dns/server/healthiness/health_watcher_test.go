package healthiness_test

import (
	"time"

	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"

	"code.cloudfoundry.org/clock/fakeclock"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthWatcher", func() {
	var (
		fakeChecker *healthinessfakes.FakeHealthChecker
		fakeClock   *fakeclock.FakeClock
		fakeLogger  *loggerfakes.FakeLogger
		interval    time.Duration
		signal      chan struct{}
		stopped     chan struct{}

		healthWatcher healthiness.HealthWatcher
	)

	BeforeEach(func() {
		fakeChecker = &healthinessfakes.FakeHealthChecker{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeLogger = &loggerfakes.FakeLogger{}
		interval = time.Second
		healthWatcher = healthiness.NewHealthWatcher(fakeChecker, fakeClock, interval, fakeLogger)
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

	Describe("HealthState", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.1"
		})

		Context("when the status is not known", func() {
			Context("and the ip is not known because it's not tracked", func() {
				It("returns unchecked", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateUnchecked))
				})
			})

			Context("and the ip is known", func() {
				JustBeforeEach(func() {
					fakeChecker.GetStatusReturns(healthiness.StateUnknown)
					healthWatcher.Track(ip)
					Eventually(fakeChecker.GetStatusCallCount).Should(Equal(1))
					Expect(fakeChecker.GetStatusArgsForCall(0)).To(Equal(ip))
				})

				It("returns unknown", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateUnknown))
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
					fakeChecker.GetStatusReturns(healthiness.StateHealthy)
				})

				It("returns healthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateHealthy))
				})
			})

			Context("and the ip is unhealthy", func() {
				BeforeEach(func() {
					ip = "127.0.0.3"
					fakeChecker.GetStatusReturns(healthiness.StateUnhealthy)
				})

				It("returns unhealthy", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateUnhealthy))
				})
			})

			Context("and the status changes", func() {
				BeforeEach(func() {
					fakeChecker.GetStatusReturns(healthiness.StateHealthy)
				})

				It("returns the new status", func() {
					Expect(healthWatcher.HealthState(ip)).To(Equal(healthiness.StateHealthy))

					fakeChecker.GetStatusReturns(healthiness.StateUnhealthy)
					healthWatcher.RunCheck(ip)

					Eventually(healthWatcher.HealthState(ip)).Should(Equal(healthiness.StateUnhealthy))
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

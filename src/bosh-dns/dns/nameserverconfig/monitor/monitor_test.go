package monitor_test

import (
	"errors"
	"time"

	"bosh-dns/dns/manager/managerfakes"
	"bosh-dns/dns/nameserverconfig/monitor"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/fakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	var (
		logger     *fakes.FakeLogger
		applier    monitor.Monitor
		dnsManager *managerfakes.FakeDNSManager
		fakeClock  *fakeclock.FakeClock
		ticker     clock.Ticker
	)

	BeforeEach(func() {
		logger = &fakes.FakeLogger{}
		dnsManager = &managerfakes.FakeDNSManager{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		ticker = fakeClock.NewTicker(time.Second)
		applier = monitor.NewMonitor(logger, dnsManager, ticker)
	})

	Describe("RunOnce", func() {
		It("should apply the configuration", func() {
			dnsManager.SetPrimaryReturns(nil)

			err := applier.RunOnce()
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsManager.SetPrimaryCallCount()).To(Equal(1))
		})

		Context("dns manager fails", func() {
			It("returns a wrapped error", func() {
				dnsManager.SetPrimaryReturns(errors.New("fake-err1"))

				err := applier.RunOnce()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
				Expect(dnsManager.SetPrimaryCallCount()).To(Equal(1))
			})
		})
	})

	Describe("Run", func() {
		var shutdown chan struct{}

		BeforeEach(func() {
			shutdown = make(chan struct{})
		})

		Context("when the configurer takes a while", func() {
			var isWaiting, stopWaiting chan struct{}
			BeforeEach(func() {
				isWaiting = make(chan struct{})
				stopWaiting = make(chan struct{})
				dnsManager.SetPrimaryStub = func() error {
					close(isWaiting)
					<-stopWaiting
					return nil
				}
			})

			It("should finish a lagging write before shutting down", func() {
				reallyDone := make(chan struct{})
				go func() {
					applier.Run(shutdown)
					close(reallyDone)
				}()
				fakeClock.Increment(time.Second)

				Eventually(isWaiting).Should(BeClosed())

				go func() { shutdown <- struct{}{} }()
				Consistently(reallyDone).ShouldNot(Receive())

				close(stopWaiting)
				Eventually(reallyDone).Should(BeClosed())
			})
		})

		It("continuously checks and applies changes", func() {
			go applier.Run(shutdown)

			fakeClock.Increment(time.Second)
			Eventually(dnsManager.SetPrimaryCallCount).Should(BeNumerically(">=", 1))
			fakeClock.Increment(time.Second)
			Eventually(dnsManager.SetPrimaryCallCount).Should(BeNumerically(">=", 2))
			fakeClock.Increment(time.Second)
			Eventually(dnsManager.SetPrimaryCallCount).Should(BeNumerically(">=", 3))

			close(shutdown)
		})
	})
})

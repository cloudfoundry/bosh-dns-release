package monitor_test

import (
	"errors"
	"time"

	"bosh-dns/dns/manager/managerfakes"
	"bosh-dns/dns/nameserverconfig/monitor"
	"github.com/cloudfoundry/bosh-utils/logger/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	var (
		logger       *fakes.FakeLogger
		address      string
		applier      monitor.Monitor
		dnsManager   *managerfakes.FakeDNSManager
		testInterval time.Duration
	)

	BeforeEach(func() {
		logger = &fakes.FakeLogger{}
		address = "some-address"
		dnsManager = &managerfakes.FakeDNSManager{}
		testInterval = 10 * time.Millisecond
		applier = monitor.NewMonitor(logger, address, dnsManager, testInterval/2)
	})

	Describe("RunOnce", func() {
		It("should apply the configuration", func() {
			dnsManager.SetPrimaryReturns(nil)

			err := applier.RunOnce()
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsManager.SetPrimaryCallCount()).To(Equal(1))
			Expect(dnsManager.SetPrimaryArgsForCall(0)).To(Equal(address))
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

		It("should stop after you issue a shutdown", func() {
			go applier.Run(shutdown)
			close(shutdown)

			// wait for shutdown
			time.Sleep(testInterval)
			callCountAfterShutdown := dnsManager.SetPrimaryCallCount()

			// check for any further calls to the checker
			time.Sleep(testInterval)

			Expect(dnsManager.SetPrimaryCallCount()).To(Equal(callCountAfterShutdown))
		})

		It("should apply changes", func() {
			go applier.Run(shutdown)

			time.Sleep(testInterval)
			Expect(dnsManager.SetPrimaryCallCount()).To(BeNumerically(">", 0))

			close(shutdown)
		})

		It("continues to check and apply changes", func() {
			go applier.Run(shutdown)

			time.Sleep(testInterval * 4)
			close(shutdown)

			Expect(dnsManager.SetPrimaryCallCount()).To(BeNumerically(">=", 3))
		})
	})
})

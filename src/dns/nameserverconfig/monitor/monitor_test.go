package monitor_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/monitor"

	"errors"
	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	"github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/handler/handlerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Monitor", func() {
	var (
		applier      Monitor
		checker      *handlerfakes.FakeHandler
		logger       *fakes.FakeLogger
		testInterval time.Duration
	)

	BeforeEach(func() {
		checker = &handlerfakes.FakeHandler{}
		logger = &fakes.FakeLogger{}
		testInterval = 500 * time.Millisecond
		applier = NewMonitor(checker, logger, testInterval/2)
	})

	Describe("RunOnce", func() {
		Context("checker says it is correct", func() {
			It("should do nothing", func() {
				checker.IsCorrectReturns(true, nil)

				err := applier.RunOnce()
				Expect(err).ToNot(HaveOccurred())
				Expect(checker.ApplyCallCount()).To(Equal(0))
			})
		})

		Context("checker says it is incorrect", func() {
			It("should apply the configuration", func() {
				checker.IsCorrectReturns(false, nil)
				checker.ApplyReturns(nil)

				err := applier.RunOnce()
				Expect(err).ToNot(HaveOccurred())
				Expect(checker.ApplyCallCount()).To(Equal(1))
				Expect(logger.InfoCallCount()).To(Equal(1))
			})
		})

		Context("checker says fs couldn't check the file", func() {
			It("should do nothing", func() {
				checker.IsCorrectReturns(false, errors.New("fake-err1"))

				err := applier.RunOnce()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
				Expect(checker.ApplyCallCount()).To(Equal(0))
			})
		})

		Context("checker says fs couldn't update the file", func() {
			It("should do nothing", func() {
				checker.IsCorrectReturns(false, nil)
				checker.ApplyReturns(errors.New("fake-err1"))

				err := applier.RunOnce()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
				Expect(checker.ApplyCallCount()).To(Equal(1))
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
			callCountAfterShutdown := checker.IsCorrectCallCount()

			// check for any further calls to the checker
			time.Sleep(testInterval)

			Expect(checker.IsCorrectCallCount()).To(Equal(callCountAfterShutdown))
		})

		It("should apply changes", func() {
			checker.IsCorrectReturns(false, nil)

			go applier.Run(shutdown)

			time.Sleep(testInterval)
			Expect(checker.ApplyCallCount()).To(BeNumerically(">", 0))

			close(shutdown)
		})

		It("continues to check and apply changes", func() {
			checker.IsCorrectReturns(true, nil)
			checker.IsCorrectReturnsOnCall(1, false, nil)
			checker.IsCorrectReturnsOnCall(3, false, nil)

			go applier.Run(shutdown)

			time.Sleep(testInterval * 6)
			close(shutdown)

			Expect(checker.ApplyCallCount()).To(Equal(2))
			Expect(checker.IsCorrectCallCount()).To(BeNumerically(">", 4))
		})

		It("does not apply changes when it is correct", func() {
			checker.IsCorrectReturns(true, nil)

			go applier.Run(shutdown)

			time.Sleep(testInterval)
			Expect(checker.ApplyCallCount()).To(Equal(0))

			close(shutdown)
		})
	})
})

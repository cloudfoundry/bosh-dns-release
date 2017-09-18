package healthexecutable_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-dns/healthcheck/healthexecutable"
	"time"

	"errors"
	"fmt"

	"code.cloudfoundry.org/clock/fakeclock"
	loggerfakes "github.com/cloudfoundry/bosh-utils/logger/fakes"
	sysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("HealthExecutableMonitor", func() {
	var (
		monitor         *healthexecutable.HealthExecutableMonitor
		logger          *loggerfakes.FakeLogger
		cmdRunner       *sysfakes.FakeCmdRunner
		clock           *fakeclock.FakeClock
		interval        time.Duration
		executablePaths []string
		signal          chan struct{}
	)

	BeforeEach(func() {
		logger = &loggerfakes.FakeLogger{}
		clock = fakeclock.NewFakeClock(time.Now())
		cmdRunner = sysfakes.NewFakeCmdRunner()
		interval = time.Millisecond
		executablePaths = []string{"e1", "e2", "e3"}
		signal = make(chan struct{})
	})

	JustBeforeEach(func() {
		monitor = healthexecutable.NewHealthExecutableMonitor(
			executablePaths,
			cmdRunner,
			clock,
			interval,
			signal,
			logger,
		)
	})

	AfterEach(func() {
		if signal != nil {
			close(signal)
		}
	})

	Context("when some executables go unhealthy and they become healthy again", func() {
		BeforeEach(func() {
			cmdRunner.AddCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})

			cmdRunner.AddCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 1})
			cmdRunner.AddCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})

			cmdRunner.AddCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})
		})

		It("starts with status true", func() {
			Expect(monitor.Status()).To(BeTrue())
		})

		It("returns status accordingly", func() {
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(BeTrue())
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(BeFalse())
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(BeTrue())
		})
	})

	Context("when executing an executable returns an error", func() {
		BeforeEach(func() {
			cmdRunner.AddCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			cmdRunner.AddCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0, Error: errors.New("can't do that")})
			cmdRunner.AddCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})
		})

		It("logs an error", func() {
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(BeFalse())

			Expect(logger.ErrorCallCount()).To(Equal(1))
			logTag, template, interpols := logger.ErrorArgsForCall(0)
			Expect(logTag).To(Equal("HealthExecutableMonitor"))
			Expect(fmt.Sprintf(template, interpols...)).To(Equal("Error occurred executing 'e2': can't do that"))
		})
	})

	Context("when no executables are defined", func() {
		BeforeEach(func() {
			executablePaths = []string{}
		})

		It("always returns status true", func() {
			clock.WaitForWatcherAndIncrement(interval)
			Consistently(monitor.Status).Should(BeTrue())
		})
	})

	Context("when shutting down", func() {
		It("stops calling the executables", func() {
			close(signal)
			signal = nil

			cmdRunner.AddCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 1})
			Eventually(clock.WatcherCount).Should(Equal(0))
			clock.Increment(interval * 2)
			Consistently(monitor.Status).Should(Equal(true))
		})
	})
})

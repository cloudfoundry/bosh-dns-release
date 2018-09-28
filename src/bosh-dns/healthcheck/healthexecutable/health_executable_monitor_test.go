package healthexecutable_test

import (
	"io/ioutil"
	"os"
	"runtime"

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
		clock                  *fakeclock.FakeClock
		cmdRunner              *sysfakes.FakeCmdRunner
		executablePaths        []string
		healthExecutablePrefix string
		healthFile             *os.File
		interval               time.Duration
		logger                 *loggerfakes.FakeLogger
		monitor                *healthexecutable.HealthExecutableMonitor
		signal                 chan struct{}
	)

	writeState := func(status string) {
		Expect(healthFile.Truncate(0)).To(Succeed())

		_, err := healthFile.Seek(0, 0)
		Expect(err).NotTo(HaveOccurred())

		_, err = healthFile.Write([]byte(fmt.Sprintf(`{"state":"%s"}`, status)))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		var err error

		logger = &loggerfakes.FakeLogger{}
		clock = fakeclock.NewFakeClock(time.Now())
		cmdRunner = sysfakes.NewFakeCmdRunner()
		interval = time.Millisecond

		healthFile, err = ioutil.TempFile("", "health-executable-state")
		Expect(err).NotTo(HaveOccurred())

		if runtime.GOOS == "windows" {
			healthExecutablePrefix = "powershell.exe "
		}

		executablePaths = []string{
			"e1",
			"e2",
			"e3",
		}
		signal = make(chan struct{})

		writeState("running")
	})

	JustBeforeEach(func() {
		monitor = healthexecutable.NewHealthExecutableMonitor(
			healthFile.Name(),
			executablePaths,
			cmdRunner,
			clock,
			interval,
			signal,
			logger,
		)
	})

	AfterEach(func() {
		Expect(healthFile.Close()).To(Succeed())
		Expect(os.RemoveAll(healthFile.Name())).To(Succeed())

		if signal != nil {
			close(signal)
		}
	})

	addCmdResult := func(executablePath string, result sysfakes.FakeCmdResult) {
		cmdRunner.AddCmdResult(healthExecutablePrefix+executablePath, result)
	}

	Context("when the agent's health file reports a failure", func() {
		BeforeEach(func() {
			executablePaths = []string{}
		})

		It("returns the unhealthy state", func() {
			Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusRunning}))

			writeState("stopped")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))

			writeState("running")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusRunning}))

			writeState("stopped")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
		})
	})

	Context("when the agent's health file is invalid", func() {
		BeforeEach(func() {
			executablePaths = []string{}
		})

		Context("with invalid json", func() {
			BeforeEach(func() {
				writeState(`{"{`)
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
			})
		})

		Context("missing file", func() {
			BeforeEach(func() {
				os.RemoveAll(healthFile.Name())
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
			})
		})
	})

	Context("when some executables go unhealthy and they become healthy again", func() {
		BeforeEach(func() {
			addCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})

			addCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 1})
			addCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})

			addCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})

			addCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 1})
		})

		It("starts with the result of the first set of commands", func() {
			Expect(cmdRunner.RunCommands).To(HaveLen(3))
			Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusRunning}))
		})

		It("returns status accordingly", func() {
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
			Eventually(cmdRunner.RunCommands).Should(HaveLen(6))
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusRunning}))
			Eventually(cmdRunner.RunCommands).Should(HaveLen(9))
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
			Eventually(cmdRunner.RunCommands).Should(HaveLen(12))
		})
	})

	Context("when executing an executable returns an error", func() {
		BeforeEach(func() {
			addCmdResult(executablePaths[0], sysfakes.FakeCmdResult{ExitStatus: 0})
			addCmdResult(executablePaths[1], sysfakes.FakeCmdResult{ExitStatus: 0, Error: errors.New("can't do that")})
			addCmdResult(executablePaths[2], sysfakes.FakeCmdResult{ExitStatus: 0})
		})

		It("logs an error", func() {
			Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
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
			Consistently(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusRunning}))
		})
	})

	Context("when shutting down", func() {
		It("stops calling the executables", func() {
			Eventually(cmdRunner.RunCommands).Should(HaveLen(3))
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
			Eventually(clock.WatcherCount).Should(Equal(1))

			close(signal)
			signal = nil

			Eventually(clock.WatcherCount).Should(Equal(0))
			clock.Increment(interval * 2)
			Consistently(cmdRunner.RunCommands).Should(HaveLen(3))
			Consistently(monitor.Status).Should(Equal(healthexecutable.HealthResult{State: healthexecutable.StatusStopped}))
		})
	})
})

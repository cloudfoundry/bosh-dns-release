package healthexecutable_test

import (
	"encoding/json"
	"io/ioutil"
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

type Health struct {
	State string `json:"state"`
}

var _ = Describe("HealthExecutableMonitor", func() {
	var (
		monitor                *healthexecutable.HealthExecutableMonitor
		healthJsonFileName     string
		recordsJsonFileName    string
		logger                 *loggerfakes.FakeLogger
		cmdRunner              *sysfakes.FakeCmdRunner
		clock                  *fakeclock.FakeClock
		interval               time.Duration
		executablePaths        []string
		signal                 chan struct{}
		healthExecutablePrefix string
		status                 string
		runningStatus          healthexecutable.Status
		stoppedStatus          healthexecutable.Status
	)

	BeforeEach(func() {
		logger = &loggerfakes.FakeLogger{}
		clock = fakeclock.NewFakeClock(time.Now())
		cmdRunner = sysfakes.NewFakeCmdRunner()
		interval = time.Millisecond

		if runtime.GOOS == "windows" {
			healthExecutablePrefix = "powershell.exe "
		}

		groupSuccess := map[string]bool{
			"q-g1.bosh": true,
			"q-g2.bosh": true,
		}
		runningStatus = healthexecutable.Status{VmStatus: true, GroupStatus: groupSuccess}

		groupFailure := map[string]bool{
			"q-g1.bosh": false,
			"q-g2.bosh": false,
		}
		stoppedStatus = healthexecutable.Status{VmStatus: false, GroupStatus: groupFailure}
		executablePaths = []string{
			"e1",
			"e2",
			"e3",
		}
		signal = make(chan struct{})
		status = "running"
	})

	JustBeforeEach(func() {
		f, err := ioutil.TempFile("", "health-executable-monitor")
		Expect(err).NotTo(HaveOccurred())
		healthJsonFileName = f.Name()
		healthRaw, err := json.Marshal(Health{State: status})
		Expect(err).ToNot(HaveOccurred())

		err = ioutil.WriteFile(healthJsonFileName, healthRaw, 0777)
		Expect(err).ToNot(HaveOccurred())

		recordsFile, err := ioutil.TempFile("", "recordsjson")
		Expect(err).NotTo(HaveOccurred())

		_, err = recordsFile.Write([]byte(fmt.Sprint(`{
			"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id","network", "deployment", "ip", "domain"],
			"record_infos": [
				["my-instance", "123", "my-group", ["1"], "az1", "1", "my-network", "my-deployment", "127.0.0.1", "bosh"],
				["my-instance-1", "456", "my-group", ["2"], "az2", "2", "my-network", "my-deployment", "127.0.0.2", "bosh"]
			]
		}`)))
		Expect(err).NotTo(HaveOccurred())

		recordsJsonFileName = recordsFile.Name()

		monitor = healthexecutable.NewHealthExecutableMonitor(
			executablePaths,
			healthJsonFileName,
			recordsJsonFileName,
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

	addCmdResult := func(executablePath string, result sysfakes.FakeCmdResult) {
		cmdRunner.AddCmdResult(healthExecutablePrefix+executablePath, result)
	}

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
			Expect(monitor.Status()).To(Equal(runningStatus))
		})

		It("returns status accordingly", func() {
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(stoppedStatus))
			Eventually(cmdRunner.RunCommands).Should(HaveLen(6))
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(runningStatus))
			Eventually(cmdRunner.RunCommands).Should(HaveLen(9))
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(stoppedStatus))
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
			Expect(monitor.Status()).To(Equal(stoppedStatus))
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
			Consistently(monitor.Status).Should(Equal(runningStatus))
		})
	})

	Context("when shutting down", func() {
		It("stops calling the executables", func() {
			Eventually(cmdRunner.RunCommands).Should(HaveLen(3))
			Eventually(monitor.Status).Should(Equal(stoppedStatus))
			Eventually(clock.WatcherCount).Should(Equal(1))

			close(signal)
			signal = nil

			Eventually(clock.WatcherCount).Should(Equal(0))
			clock.Increment(interval * 2)
			Consistently(cmdRunner.RunCommands).Should(HaveLen(3))
			Consistently(monitor.Status).Should(Equal(stoppedStatus))
		})
	})
})

package healthexecutable_test

import (
	"io/ioutil"
	"os"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-dns/healthcheck/healthconfig"
	"bosh-dns/healthcheck/healthexecutable"
	"time"

	"errors"
	"fmt"

	"code.cloudfoundry.org/clock/fakeclock"
	loggerfakes "github.com/cloudfoundry/bosh-utils/logger/fakes"
	sysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("Monitor", func() {
	var (
		clock                  *fakeclock.FakeClock
		cmdRunner              *sysfakes.FakeCmdRunner
		jobs                   []healthconfig.Job
		healthExecutablePrefix string
		healthFile             *os.File
		interval               time.Duration
		logger                 *loggerfakes.FakeLogger
		monitor                *healthexecutable.Monitor
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

		jobs = []healthconfig.Job{}

		signal = make(chan struct{})

		writeState("running")
	})

	JustBeforeEach(func() {
		monitor = healthexecutable.NewMonitor(
			healthFile.Name(),
			jobs,
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

	It("returns status true", func() {
		clock.WaitForWatcherAndIncrement(interval)
		Consistently(monitor.Status).Should(Equal(healthexecutable.HealthResult{
			State:      healthexecutable.StatusRunning,
			GroupState: make(map[string]healthexecutable.HealthStatus),
		}))
	})

	Context("when the agent's health file reports a failure", func() {
		It("returns the unhealthy state", func() {
			Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
				State:      healthexecutable.StatusRunning,
				GroupState: make(map[string]healthexecutable.HealthStatus),
			}))

			writeState("failing")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
				State:      healthexecutable.StatusFailing,
				GroupState: make(map[string]healthexecutable.HealthStatus),
			}))

			writeState("running")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
				State:      healthexecutable.StatusRunning,
				GroupState: make(map[string]healthexecutable.HealthStatus),
			}))

			writeState("failing")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
				State:      healthexecutable.StatusFailing,
				GroupState: make(map[string]healthexecutable.HealthStatus),
			}))
		})
	})

	Context("when the agent's health file is invalid", func() {
		Context("with invalid json", func() {
			BeforeEach(func() {
				writeState(`{"{`)
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
			})
		})

		Context("missing file", func() {
			BeforeEach(func() {
				os.RemoveAll(healthFile.Name())
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
			})
		})
	})

	Context("when jobs have executables defined", func() {
		BeforeEach(func() {
			jobs = []healthconfig.Job{
				{HealthExecutablePath: "e1"},
				{HealthExecutablePath: "e2"},
				{HealthExecutablePath: "e3"},
			}
		})

		Context("when some executables go unhealthy and they become healthy again", func() {
			BeforeEach(func() {
				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[2].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})
				addCmdResult(jobs[2].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[2].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[2].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})
			})

			It("starts with the result of the first set of commands", func() {
				Expect(cmdRunner.RunCommands).To(HaveLen(3))
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusRunning,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
			})

			It("returns status accordingly", func() {
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
				Eventually(cmdRunner.RunCommands).Should(HaveLen(6))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusRunning,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
				Eventually(cmdRunner.RunCommands).Should(HaveLen(9))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
				Eventually(cmdRunner.RunCommands).Should(HaveLen(12))
			})
		})

		Context("when executing an executable returns an error", func() {
			BeforeEach(func() {
				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0, Error: errors.New("can't do that")})
				addCmdResult(jobs[2].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
			})

			It("logs an error", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
				Expect(logger.ErrorCallCount()).To(Equal(1))
				logTag, template, interpols := logger.ErrorArgsForCall(0)
				Expect(logTag).To(Equal("Monitor"))
				Expect(fmt.Sprintf(template, interpols...)).To(Equal("Error occurred executing 'e2': can't do that"))
			})
		})

		Context("when shutting down", func() {
			It("stops calling the executables", func() {
				Eventually(cmdRunner.RunCommands).Should(HaveLen(3))
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
				Eventually(clock.WatcherCount).Should(Equal(1))

				close(signal)
				signal = nil

				Eventually(clock.WatcherCount).Should(Equal(0))
				clock.Increment(interval * 2)
				Consistently(cmdRunner.RunCommands).Should(HaveLen(3))
				Consistently(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State:      healthexecutable.StatusFailing,
					GroupState: make(map[string]healthexecutable.HealthStatus),
				}))
			})
		})
	})

	Context("when there are groups present", func() {
		Context("and the groups have no executables", func() {
			BeforeEach(func() {
				jobs = []healthconfig.Job{
					{HealthExecutablePath: "", Groups: []string{"1"}},
					{HealthExecutablePath: "", Groups: []string{"2"}},
					{HealthExecutablePath: "", Groups: []string{"3"}},
				}
			})

			It("reports the VM state as the group status", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusRunning,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusRunning,
						"2": healthexecutable.StatusRunning,
						"3": healthexecutable.StatusRunning,
					},
				}))

				writeState("failing")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusFailing,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusFailing,
						"2": healthexecutable.StatusFailing,
						"3": healthexecutable.StatusFailing,
					},
				}))

				writeState("running")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusRunning,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusRunning,
						"2": healthexecutable.StatusRunning,
						"3": healthexecutable.StatusRunning,
					},
				}))

				writeState("failing")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusFailing,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusFailing,
						"2": healthexecutable.StatusFailing,
						"3": healthexecutable.StatusFailing,
					},
				}))
			})
		})

		Context("and the groups have executables", func() {
			BeforeEach(func() {
				jobs = []healthconfig.Job{
					{HealthExecutablePath: "e1", Groups: []string{"1"}},
					{HealthExecutablePath: "e2", Groups: []string{"2"}},
					{HealthExecutablePath: "", Groups: []string{"3"}},
				}

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})

				addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})
				addCmdResult(jobs[1].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
			})

			It("reports the executable status for each group", func() {
				Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusRunning,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusRunning,
						"2": healthexecutable.StatusRunning,
						"3": healthexecutable.StatusRunning,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusFailing,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusRunning,
						"2": healthexecutable.StatusFailing,
						"3": healthexecutable.StatusFailing,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusRunning,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusRunning,
						"2": healthexecutable.StatusRunning,
						"3": healthexecutable.StatusRunning,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
					State: healthexecutable.StatusFailing,
					GroupState: map[string]healthexecutable.HealthStatus{
						"1": healthexecutable.StatusFailing,
						"2": healthexecutable.StatusRunning,
						"3": healthexecutable.StatusFailing,
					},
				}))
			})

			Context("when there are duplicate executables across groups", func() {
				BeforeEach(func() {
					jobs = []healthconfig.Job{
						{HealthExecutablePath: "duplicate-executable", Groups: []string{"1"}},
						{HealthExecutablePath: "duplicate-executable", Groups: []string{"2"}},
					}

					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})
					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				})

				It("only executes the executable once", func() {
					Expect(monitor.Status()).To(Equal(healthexecutable.HealthResult{
						State: healthexecutable.StatusRunning,
						GroupState: map[string]healthexecutable.HealthStatus{
							"1": healthexecutable.StatusRunning,
							"2": healthexecutable.StatusRunning,
						},
					}))

					clock.WaitForWatcherAndIncrement(interval)
					Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
						State: healthexecutable.StatusFailing,
						GroupState: map[string]healthexecutable.HealthStatus{
							"1": healthexecutable.StatusFailing,
							"2": healthexecutable.StatusFailing,
						},
					}))

					clock.WaitForWatcherAndIncrement(interval)
					Eventually(monitor.Status).Should(Equal(healthexecutable.HealthResult{
						State: healthexecutable.StatusRunning,
						GroupState: map[string]healthexecutable.HealthStatus{
							"1": healthexecutable.StatusRunning,
							"2": healthexecutable.StatusRunning,
						},
					}))
				})
			})
		})
	})
})

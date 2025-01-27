package healthexecutable_test

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	sysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/healthcheck/api"
	"bosh-dns/healthcheck/healthexecutable"
	"bosh-dns/healthconfig"
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
		h, err := os.OpenFile(healthFile.Name(), os.O_WRONLY, 0644)
		Expect(err).ToNot(HaveOccurred())

		_, err = h.Write([]byte(fmt.Sprintf(`{"state":"%s"}`, status)))
		Expect(err).NotTo(HaveOccurred())
		Expect(h.Close()).To(Succeed())
	}

	BeforeEach(func() {
		var err error

		logger = &loggerfakes.FakeLogger{}
		clock = fakeclock.NewFakeClock(time.Now())
		cmdRunner = sysfakes.NewFakeCmdRunner()
		interval = time.Millisecond

		healthFile, err = os.CreateTemp("", "health-executable-state")
		Expect(err).NotTo(HaveOccurred())
		Expect(healthFile.Close()).To(Succeed())

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
		Consistently(monitor.Status).Should(Equal(api.HealthResult{
			State:      api.StatusRunning,
			GroupState: make(map[string]api.HealthStatus),
		}))
	})

	Context("when the agent's health file reports a failure", func() {
		It("returns the unhealthy state", func() {
			Expect(monitor.Status()).To(Equal(api.HealthResult{
				State:      api.StatusRunning,
				GroupState: make(map[string]api.HealthStatus),
			}))

			writeState("failing")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(api.HealthResult{
				State:      api.StatusFailing,
				GroupState: make(map[string]api.HealthStatus),
			}))

			writeState("running")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(api.HealthResult{
				State:      api.StatusRunning,
				GroupState: make(map[string]api.HealthStatus),
			}))

			writeState("failing")
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(monitor.Status).Should(Equal(api.HealthResult{
				State:      api.StatusFailing,
				GroupState: make(map[string]api.HealthStatus),
			}))
		})
	})

	Context("when the agent's health file is invalid", func() {
		Context("with invalid json", func() {
			BeforeEach(func() {
				writeState(`{"{`)
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
				}))
			})
		})

		Context("missing file", func() {
			BeforeEach(func() {
				err := os.RemoveAll(healthFile.Name())
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the unhealthy state", func() {
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
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
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State:      api.StatusRunning,
					GroupState: make(map[string]api.HealthStatus),
				}))
			})

			It("returns status accordingly", func() {
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
				}))
				Eventually(cmdRunner.RunCommands).Should(HaveLen(6))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State:      api.StatusRunning,
					GroupState: make(map[string]api.HealthStatus),
				}))
				Eventually(cmdRunner.RunCommands).Should(HaveLen(9))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
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
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
				}))
				Expect(logger.WarnCallCount()).To(Equal(1))
				logTag, template, interpols := logger.WarnArgsForCall(0)
				Expect(logTag).To(Equal("Monitor"))
				Expect(fmt.Sprintf(template, interpols...)).To(Equal("Error occurred executing 'e2': can't do that"))
			})
		})

		Context("when shutting down", func() {
			It("stops calling the executables", func() {
				Eventually(cmdRunner.RunCommands).Should(HaveLen(3))
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
				}))
				Eventually(clock.WatcherCount).Should(Equal(1))

				close(signal)
				signal = nil

				Eventually(clock.WatcherCount).Should(Equal(0))
				clock.Increment(interval * 2)
				Consistently(cmdRunner.RunCommands).Should(HaveLen(3))
				Consistently(monitor.Status).Should(Equal(api.HealthResult{
					State:      api.StatusFailing,
					GroupState: make(map[string]api.HealthStatus),
				}))
			})
		})
	})

	Context("when there are groups present", func() {
		Context("and the groups have no executables", func() {
			BeforeEach(func() {
				jobs = []healthconfig.Job{
					{HealthExecutablePath: "", Groups: []healthconfig.LinkMetadata{{Group: "1"}}},
					{HealthExecutablePath: "", Groups: []healthconfig.LinkMetadata{{Group: "2"}}},
					{HealthExecutablePath: "", Groups: []healthconfig.LinkMetadata{{Group: "3"}}},
				}
			})

			It("reports the VM state as the group status", func() {
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State: api.StatusRunning,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusRunning,
						"2": api.StatusRunning,
						"3": api.StatusRunning,
					},
				}))

				writeState("failing")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusFailing,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusFailing,
						"2": api.StatusFailing,
						"3": api.StatusFailing,
					},
				}))

				writeState("running")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusRunning,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusRunning,
						"2": api.StatusRunning,
						"3": api.StatusRunning,
					},
				}))

				writeState("failing")
				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusFailing,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusFailing,
						"2": api.StatusFailing,
						"3": api.StatusFailing,
					},
				}))
			})
		})

		Context("and the groups have executables", func() {
			BeforeEach(func() {
				jobs = []healthconfig.Job{
					{HealthExecutablePath: "e1", Groups: []healthconfig.LinkMetadata{{Group: "1"}}},
					{HealthExecutablePath: "e2", Groups: []healthconfig.LinkMetadata{{Group: "2"}}},
					{HealthExecutablePath: "", Groups: []healthconfig.LinkMetadata{{Group: "3"}}},
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
				Expect(monitor.Status()).To(Equal(api.HealthResult{
					State: api.StatusRunning,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusRunning,
						"2": api.StatusRunning,
						"3": api.StatusRunning,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusFailing,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusRunning,
						"2": api.StatusFailing,
						"3": api.StatusFailing,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusRunning,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusRunning,
						"2": api.StatusRunning,
						"3": api.StatusRunning,
					},
				}))

				clock.WaitForWatcherAndIncrement(interval)
				Eventually(monitor.Status).Should(Equal(api.HealthResult{
					State: api.StatusFailing,
					GroupState: map[string]api.HealthStatus{
						"1": api.StatusFailing,
						"2": api.StatusRunning,
						"3": api.StatusFailing,
					},
				}))
			})

			Context("when there are duplicate executables across groups", func() {
				BeforeEach(func() {
					jobs = []healthconfig.Job{
						{HealthExecutablePath: "duplicate-executable", Groups: []healthconfig.LinkMetadata{{Group: "1"}}},
						{HealthExecutablePath: "duplicate-executable", Groups: []healthconfig.LinkMetadata{{Group: "2"}}},
					}

					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 1})
					addCmdResult(jobs[0].HealthExecutablePath, sysfakes.FakeCmdResult{ExitStatus: 0})
				})

				It("only executes the executable once", func() {
					Expect(monitor.Status()).To(Equal(api.HealthResult{
						State: api.StatusRunning,
						GroupState: map[string]api.HealthStatus{
							"1": api.StatusRunning,
							"2": api.StatusRunning,
						},
					}))

					clock.WaitForWatcherAndIncrement(interval)
					Eventually(monitor.Status).Should(Equal(api.HealthResult{
						State: api.StatusFailing,
						GroupState: map[string]api.HealthStatus{
							"1": api.StatusFailing,
							"2": api.StatusFailing,
						},
					}))

					clock.WaitForWatcherAndIncrement(interval)
					Eventually(monitor.Status).Should(Equal(api.HealthResult{
						State: api.StatusRunning,
						GroupState: map[string]api.HealthStatus{
							"1": api.StatusRunning,
							"2": api.StatusRunning,
						},
					}))
				})
			})
		})
	})
})

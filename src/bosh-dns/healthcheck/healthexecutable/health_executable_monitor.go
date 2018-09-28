package healthexecutable

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"sync"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type HealthStatus string

const (
	StatusRunning HealthStatus = "running"
	StatusStopped HealthStatus = "stopped"
)

type HealthResult struct {
	State HealthStatus
}

type HealthExecutableMonitor struct {
	clock                 clock.Clock
	cmdRunner             system.CmdRunner
	healthExecutablePaths []string
	healthFilePath        string
	interval              time.Duration
	logger                logger.Logger
	mutex                 *sync.Mutex
	shutdown              chan struct{}
	status                HealthResult
}

func NewHealthExecutableMonitor(
	healthFilePath string,
	healthExecutablePaths []string,
	cmdRunner system.CmdRunner,
	clock clock.Clock,
	interval time.Duration,
	shutdown chan struct{},
	logger logger.Logger,
) *HealthExecutableMonitor {
	monitor := &HealthExecutableMonitor{
		clock:                 clock,
		cmdRunner:             cmdRunner,
		healthExecutablePaths: healthExecutablePaths,
		healthFilePath:        healthFilePath,
		interval:              interval,
		logger:                logger,
		mutex:                 &sync.Mutex{},
		shutdown:              shutdown,
		status: HealthResult{
			State: StatusStopped,
		},
	}

	monitor.runChecks()
	go monitor.run()

	return monitor
}

func (m *HealthExecutableMonitor) Status() HealthResult {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.status
}

func (m *HealthExecutableMonitor) run() {
	timer := m.clock.NewTimer(m.interval)
	m.logger.Debug("HealthExecutableMonitor", "starting monitor for [%s] with interval %v", strings.Join(m.healthExecutablePaths, ", "), m.interval)

	for {
		select {
		case <-m.shutdown:
			m.logger.Debug("HealthExecutableMonitor", "stopping")
			timer.Stop()
			return
		case <-timer.C():
			m.runChecks()
			timer.Reset(m.interval)
		}
	}
}

type agentHealth struct {
	State HealthStatus `json:"state"`
}

func (m *HealthExecutableMonitor) runChecks() {
	err := m.readAgentHealth()
	if err != nil {
		m.mutex.Lock()
		m.status.State = StatusStopped
		m.mutex.Unlock()
		return
	}

	allStatus := StatusRunning
	for _, executable := range m.healthExecutablePaths {
		_, _, exitStatus, err := m.runExecutable(executable)
		if err != nil {
			allStatus = StatusStopped
			m.logger.Error("HealthExecutableMonitor", "Error occurred executing '%s': %v", executable, err)
		} else if exitStatus != 0 {
			allStatus = StatusStopped
		}
	}

	m.mutex.Lock()
	m.status.State = allStatus
	m.mutex.Unlock()
}

func (m *HealthExecutableMonitor) readAgentHealth() error {
	data, err := ioutil.ReadFile(m.healthFilePath)
	if err != nil {
		return err
	}

	var agentHealthResult agentHealth
	err = json.Unmarshal(data, &agentHealthResult)
	if err != nil {
		return err
	}

	if agentHealthResult.State != StatusRunning {
		return errors.New("state is not running")
	}

	return nil
}

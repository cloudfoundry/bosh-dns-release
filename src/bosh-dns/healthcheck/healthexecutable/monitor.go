package healthexecutable

import (
	"bosh-dns/healthcheck/healthconfig"
	"encoding/json"
	"io/ioutil"
	"time"

	"sync"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type HealthStatus string

const (
	StatusRunning HealthStatus = "running"
	StatusFailing HealthStatus = "failing"
)

type HealthResult struct {
	State      HealthStatus            `json:"state"`
	GroupState map[string]HealthStatus `json:"group_state,omitempty"`
}

type agentHealth struct {
	State HealthStatus `json:"state"`
}

type Monitor struct {
	clock          clock.Clock
	cmdRunner      system.CmdRunner
	healthFilePath string
	interval       time.Duration
	jobs           []healthconfig.Job
	logger         logger.Logger
	mutex          *sync.Mutex
	shutdown       chan struct{}
	status         HealthResult
}

func NewMonitor(
	healthFilePath string,
	jobs []healthconfig.Job,
	cmdRunner system.CmdRunner,
	clock clock.Clock,
	interval time.Duration,
	shutdown chan struct{},
	logger logger.Logger,
) *Monitor {
	monitor := &Monitor{
		clock:          clock,
		cmdRunner:      cmdRunner,
		healthFilePath: healthFilePath,
		interval:       interval,
		jobs:           jobs,
		logger:         logger,
		mutex:          &sync.Mutex{},
		shutdown:       shutdown,
		status: HealthResult{
			State: StatusFailing,
		},
	}

	monitor.runChecks()
	go monitor.run()

	return monitor
}

func (m *Monitor) Status() HealthResult {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.status
}

func (m *Monitor) run() {
	timer := m.clock.NewTimer(m.interval)
	m.logger.Debug("Monitor", "starting monitor with interval %v", m.interval)

	for {
		select {
		case <-m.shutdown:
			m.logger.Debug("Monitor", "stopping")
			timer.Stop()
			return
		case <-timer.C():
			m.runChecks()
			timer.Reset(m.interval)
		}
	}
}

func (m *Monitor) runChecks() {
	agentStatus := m.readAgentHealth()

	groupState := make(map[string]HealthStatus)
	groupsWithoutExecutable := []string{}
	checkedResults := make(map[string]HealthStatus)

	allStatus := agentStatus
	for _, job := range m.jobs {
		if job.HealthExecutablePath == "" {
			groupsWithoutExecutable = append(groupsWithoutExecutable, job.Groups...)
			continue
		}

		var executableStatus HealthStatus
		var ok bool
		if executableStatus, ok = checkedResults[job.HealthExecutablePath]; !ok {
			executableStatus = m.executableStatus(job.HealthExecutablePath)
			checkedResults[job.HealthExecutablePath] = executableStatus
		}

		setStateForGroupIDs(groupState, job.Groups, executableStatus)

		if notRunning(executableStatus) {
			allStatus = executableStatus
		}
	}

	setStateForGroupIDs(groupState, groupsWithoutExecutable, allStatus)

	m.setHealthResult(allStatus, groupState)
}

func (m *Monitor) setHealthResult(status HealthStatus, groupState map[string]HealthStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.status = HealthResult{State: status, GroupState: groupState}
}

func (m *Monitor) executableStatus(executablePath string) HealthStatus {
	_, _, exitStatus, err := m.runExecutable(executablePath)
	if err != nil {
		m.logger.Error("Monitor", "Error occurred executing '%s': %s", executablePath, err.Error())
		return StatusFailing
	}

	if exitStatus != 0 {
		return StatusFailing
	}

	return StatusRunning
}

func (m *Monitor) readAgentHealth() HealthStatus {
	data, err := ioutil.ReadFile(m.healthFilePath)
	if err != nil {
		return StatusFailing
	}

	var agentHealthResult agentHealth
	err = json.Unmarshal(data, &agentHealthResult)
	if err != nil {
		return StatusFailing
	}

	if notRunning(agentHealthResult.State) {
		return StatusFailing
	}

	return StatusRunning
}

func setStateForGroupIDs(groupState map[string]HealthStatus, groupIDs []string, status HealthStatus) {
	for _, groupID := range groupIDs {
		groupState[groupID] = status
	}
}

func notRunning(status HealthStatus) bool {
	return status != StatusRunning
}

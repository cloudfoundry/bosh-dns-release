package healthexecutable

import (
	"bosh-dns/healthcheck/api"
	"bosh-dns/healthconfig"
	"encoding/json"
	"io/ioutil" //nolint:staticcheck
	"time"

	"sync"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type agentHealth struct {
	State api.HealthStatus `json:"state"`
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
	status         api.HealthResult
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
		status: api.HealthResult{
			State: api.StatusFailing,
		},
	}

	monitor.runChecks()
	go monitor.run()

	return monitor
}

func (m *Monitor) Status() api.HealthResult {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.status
}

func (m *Monitor) run() {
	timer := m.clock.NewTimer(m.interval)
	m.logger.Debug("Monitor", "starting monitor with interval %v", m.interval)
	m.logger.Info("Monitor", "Initial status: %s", m.status.State)
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

	groupState := make(map[string]api.HealthStatus)
	groupsWithoutExecutable := []healthconfig.LinkMetadata{}
	checkedResults := make(map[string]api.HealthStatus)

	allStatus := agentStatus
	for _, job := range m.jobs {
		if job.HealthExecutablePath == "" {
			groupsWithoutExecutable = append(groupsWithoutExecutable, job.Groups...)
			continue
		}

		var executableStatus api.HealthStatus
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

	m.logger.Debug("Monitor", "Health status: %+v", allStatus)
	m.logger.Debug("Monitor", "Group state: %+v", groupState)
	oldStatus := m.status
	m.setHealthResult(allStatus, groupState)
	if oldStatus.State != m.status.State {
		m.logger.Info("Monitor", "Status changed from %s to %s", oldStatus.State, m.status.State)
	}
}

func (m *Monitor) setHealthResult(status api.HealthStatus, groupState map[string]api.HealthStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.status = api.HealthResult{State: status, GroupState: groupState}
}

func (m *Monitor) executableStatus(executablePath string) api.HealthStatus {
	stdout, stderr, exitStatus, err := m.runExecutable(executablePath)
	m.logger.Debug("Monitor", "Script %s stdout: %s", executablePath, stdout)
	m.logger.Debug("Monitor", "Script %s stderr: %s", executablePath, stderr)
	if err != nil {
		m.logger.Warn("Monitor", "Error occurred executing '%s': %s", executablePath, err.Error())
		return api.StatusFailing
	}

	if exitStatus != 0 {
		m.logger.Warn("Monitor", "Error occurred executing '%s': %s", executablePath, exitStatus)
		return api.StatusFailing
	}

	m.logger.Debug("Monitor", "Script %s completed successfully", executablePath)
	return api.StatusRunning
}

func (m *Monitor) readAgentHealth() api.HealthStatus {
	data, err := ioutil.ReadFile(m.healthFilePath)
	if err != nil {
		m.logger.Error("Monitor", "Error reading health file: %s", err.Error())
		return api.StatusFailing
	}

	var agentHealthResult agentHealth
	err = json.Unmarshal(data, &agentHealthResult)
	if err != nil {
		m.logger.Error("Monitor", "Error parsing health file: %s", err.Error())
		return api.StatusFailing
	}

	if notRunning(agentHealthResult.State) {
		m.logger.Warn("Monitor", "Agent reports unhealthy: %+v", agentHealthResult)
		return api.StatusFailing
	}

	m.logger.Debug("Monitor", "Agent reports healthy: %+v", agentHealthResult)
	return api.StatusRunning
}

func setStateForGroupIDs(groupState map[string]api.HealthStatus, linkMetadata []healthconfig.LinkMetadata, status api.HealthStatus) {
	for _, linkMetadatum := range linkMetadata {
		groupState[linkMetadatum.Group] = status
	}
}

func notRunning(status api.HealthStatus) bool {
	return status != api.StatusRunning
}

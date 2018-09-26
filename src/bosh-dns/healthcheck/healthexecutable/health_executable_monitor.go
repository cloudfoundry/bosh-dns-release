package healthexecutable

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"sync"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type HealthExecutableMonitor struct {
	healthExecutablePaths []string
	healthJsonFileName    string
	recordsJsonFileName   string
	cmdRunner             system.CmdRunner
	clock                 clock.Clock
	interval              time.Duration
	shutdown              chan struct{}
	status                bool
	groupStatus           map[string]bool
	mutex                 *sync.Mutex
	logger                logger.Logger
}

type Status struct {
	VmStatus    bool
	GroupStatus map[string]bool
}

const logTag = "HealthExecutableMonitor"

func NewHealthExecutableMonitor(
	healthExecutablePaths []string,
	healthJsonFileName string,
	recordsJsonFileName string,
	cmdRunner system.CmdRunner,
	clock clock.Clock,
	interval time.Duration,
	shutdown chan struct{},
	logger logger.Logger,
) *HealthExecutableMonitor {
	monitor := &HealthExecutableMonitor{
		healthExecutablePaths: healthExecutablePaths,
		healthJsonFileName:    healthJsonFileName,
		recordsJsonFileName:   recordsJsonFileName,
		cmdRunner:             cmdRunner,
		clock:                 clock,
		interval:              interval,
		shutdown:              shutdown,
		mutex:                 &sync.Mutex{},
		logger:                logger,
	}

	monitor.runChecks()
	go monitor.run()

	return monitor
}

func (m *HealthExecutableMonitor) Status() Status {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return Status{VmStatus: m.status, GroupStatus: m.groupStatus}
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

func (m *HealthExecutableMonitor) readHealthStatusFromFile() string {
	healthRaw, err := ioutil.ReadFile(m.healthJsonFileName)
	if err != nil {
		m.logger.Error(logTag, "Failed to read healthcheck data %s. error: %s", healthRaw, err)
		return ""
	}

	var health struct {
		State string `json:"state"`
	}

	err = json.Unmarshal(healthRaw, &health)
	if err != nil {
		m.logger.Error(logTag, "Failed to unmarshal healthcheck data %s. error: %s", healthRaw, err)
	}
	return health.State
}

func (m *HealthExecutableMonitor) allExecutablesAreHealthy() bool {
	allSucceeded := true
	for _, executable := range m.healthExecutablePaths {
		_, _, exitStatus, err := m.runExecutable(executable)
		if err != nil {
			allSucceeded = false
			m.logger.Error("HealthExecutableMonitor", "Error occurred executing '%s': %v", executable, err)
		} else if exitStatus != 0 {
			allSucceeded = false
		}
	}
	return allSucceeded
}

func (m *HealthExecutableMonitor) constructGroupStatuses(status bool) map[string]bool {
	groupStatus := map[string]bool{}

	groupHealthRaw, err := ioutil.ReadFile(m.recordsJsonFileName)
	if err != nil {
		status = false
		m.logger.Error("HealthExecutableMonitor", "Error occurred reading records file '%s'", m.recordsJsonFileName)
	}

	var records struct {
		RecordInfos [][]interface{} `json:"record_infos"`
	}

	err = json.Unmarshal(groupHealthRaw, &records)

	for _, record := range records.RecordInfos {
		groupIDs := record[3].([]interface{})
		for _, id := range groupIDs {
			group := fmt.Sprintf("q-g%s.bosh", id)
			groupStatus[group] = status
		}
	}

	return groupStatus
}

func (m *HealthExecutableMonitor) runChecks() {
	statusIsHealthy := m.readHealthStatusFromFile() == "running"
	overallStatus := statusIsHealthy && m.allExecutablesAreHealthy()

	groupStatus := m.constructGroupStatuses(overallStatus)

	m.mutex.Lock()
	m.status = overallStatus
	m.groupStatus = groupStatus
	m.mutex.Unlock()
}

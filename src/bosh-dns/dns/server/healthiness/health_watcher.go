package healthiness

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	GetStatus(ip string) bool
}

//go:generate counterfeiter . HealthWatcher

type HealthWatcher interface {
	IsHealthy(ip string) *bool
	HealthState(ip string) string
	Track(ip string)
	Untrack(ip string)
	Run(signal <-chan struct{})
}

type healthWatcher struct {
	checker       HealthChecker
	checkInterval time.Duration
	clock         clock.Clock

	checkWorkPool *workpool.WorkPool
	state         map[string]bool
	stateMutex    *sync.RWMutex
	logger        boshlog.Logger
}

func NewHealthWatcher(checker HealthChecker, clock clock.Clock, checkInterval time.Duration, logger boshlog.Logger) *healthWatcher {
	wp, _ := workpool.NewWorkPool(1000)

	return &healthWatcher{
		checker:       checker,
		checkInterval: checkInterval,
		clock:         clock,

		checkWorkPool: wp,
		state:         map[string]bool{},
		stateMutex:    &sync.RWMutex{},
		logger:        logger,
	}
}

func (hw *healthWatcher) Track(ip string) {
	hw.checkWorkPool.Submit(func() {
		hw.runCheck(ip)
	})
}

func (hw *healthWatcher) IsHealthy(ip string) *bool {
	hw.stateMutex.RLock()
	defer hw.stateMutex.RUnlock()

	if health, found := hw.state[ip]; found {
		return &health
	}
	result := true

	return &result
}

func (hw *healthWatcher) HealthState(ip string) string {
	hw.stateMutex.RLock()
	health, found := hw.state[ip]
	hw.stateMutex.RUnlock()

	if !found {
		return StateUnknown
	}

	return healthString(health)
}

func (hw *healthWatcher) Untrack(ip string) {
	hw.stateMutex.Lock()
	delete(hw.state, ip)
	hw.stateMutex.Unlock()
}

func (hw *healthWatcher) Run(signal <-chan struct{}) {
	timer := hw.clock.NewTimer(hw.checkInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C():
			hw.stateMutex.RLock()
			for ip := range hw.state {
				// closing on ip, we need to ensure it's fixed within this context
				ip := ip
				hw.checkWorkPool.Submit(func() {
					hw.runCheck(ip)
				})
			}
			hw.stateMutex.RUnlock()

			timer.Reset(hw.checkInterval)
		case <-signal:
			return
		}
	}
}

func (hw *healthWatcher) runCheck(ip string) {
	isHealthy := hw.checker.GetStatus(ip)

	hw.stateMutex.Lock()

	wasHealthy, found := hw.state[ip]
	hw.state[ip] = isHealthy

	hw.stateMutex.Unlock()

	oldState := healthString(wasHealthy)
	newState := healthString(isHealthy)

	if !found {
		hw.logger.Debug("healthWatcher", "Initial state for IP <%s> is %s", ip, oldState)
	} else if oldState != newState {
		hw.logger.Debug("healthWatcher", "State for IP <%s> changed from %s to %s", ip, oldState, newState)
	}
}

func healthString(healthy bool) string {
	if healthy {
		return StateHealthy
	}

	return StateUnhealthy
}

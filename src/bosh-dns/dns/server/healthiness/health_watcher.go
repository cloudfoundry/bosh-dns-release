package healthiness

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-dns/healthcheck/api"
)

//counterfeiter:generate . HealthChecker

type HealthChecker interface {
	GetStatus(ip string) api.HealthResult
}

//counterfeiter:generate . HealthWatcher

type HealthWatcher interface {
	HealthState(ip string) api.HealthResult
	HealthStateString(ip string) string
	Track(ip string)
	Untrack(ip string)
	Run(signal <-chan struct{})
	RunCheck(ip string) api.HealthResult
}

type healthWatcher struct {
	checker       HealthChecker
	checkInterval time.Duration
	clock         clock.Clock
	workpoolSize  int

	checkWorkPool *workpool.WorkPool
	state         map[string]api.HealthResult
	currentChecks map[string]*sync.Cond
	stateMutex    *sync.RWMutex
	logger        boshlog.Logger
}

func NewHealthWatcher(workpoolSize int, checker HealthChecker, clock clock.Clock, checkInterval time.Duration, logger boshlog.Logger) *healthWatcher {
	wp, _ := workpool.NewWorkPool(workpoolSize)

	return &healthWatcher{
		checker:       checker,
		checkInterval: checkInterval,
		clock:         clock,
		workpoolSize:  workpoolSize,

		checkWorkPool: wp,
		state:         map[string]api.HealthResult{},
		currentChecks: map[string]*sync.Cond{},
		stateMutex:    &sync.RWMutex{},
		logger:        logger,
	}
}

func (hw *healthWatcher) Track(ip string) {
	hw.checkWorkPool.Submit(func() {

		hw.stateMutex.RLock()
		_, found := hw.state[ip]
		hw.stateMutex.RUnlock()
		if found {
			hw.logger.Debug("healthWatcher", "Track found state for IP %s - wait for interval", ip)
		} else {
			hw.logger.Debug("healthWatcher", "Track check for IP %s", ip)
			hw.RunCheck(ip)
		}
	})
}

func (hw *healthWatcher) HealthStateString(ip string) string {
	return string(hw.HealthState(ip).State)
}

func (hw *healthWatcher) HealthState(ip string) api.HealthResult {
	hw.stateMutex.RLock()
	health, found := hw.state[ip]
	hw.stateMutex.RUnlock()

	if !found {
		return api.HealthResult{State: StateUnchecked}
	}
	return health
}

func (hw *healthWatcher) Untrack(ip string) {
	hw.logger.Debug("healthWatcher", "Untrack IP %s", ip)
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
			works := []func(){}

			hw.stateMutex.RLock()
			for ip := range hw.state {
				// closing on ip, we need to ensure it's fixed within this context
				ip := ip

				works = append(works, func() {
					hw.RunCheck(ip)
				})
			}
			hw.stateMutex.RUnlock()

			throttler, _ := workpool.NewThrottler(hw.workpoolSize, works)
			throttler.Work()

			timer.Reset(hw.checkInterval)
		case <-signal:
			return
		}
	}
}

func (hw *healthWatcher) RunCheck(ip string) api.HealthResult {
	hw.stateMutex.Lock()
	cond := hw.currentChecks[ip]
	if cond != nil {
		hw.logger.Debug("healthWatcher", "Request already in flight for IP %s", ip)
		cond.Wait()
		// pending request has either updated hw.state or failed
		result := hw.state[ip]
		hw.stateMutex.Unlock()
		return result
	}
	cond = sync.NewCond(hw.stateMutex)
	hw.currentChecks[ip] = cond
	hw.stateMutex.Unlock()
	healthInfo := hw.checker.GetStatus(ip)
	hw.stateMutex.Lock()
	hw.currentChecks[ip] = nil

	wasHealthy, found := hw.state[ip]
	hw.state[ip] = healthInfo

	oldState := wasHealthy
	newState := healthInfo

	if !found {
		hw.logger.Info("healthWatcher", "Initial state for IP <%s> is %s", ip, newState.State)
	} else if oldState.State != newState.State {
		hw.logger.Info("healthWatcher", "State for IP <%s> changed from %s to %s", ip, oldState.State, newState.State)
	}
	cond.Broadcast() // wake other threads waiting on this update

	hw.stateMutex.Unlock()
	return newState
}

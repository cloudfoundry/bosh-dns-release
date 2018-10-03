package healthiness

import (
	"bosh-dns/healthcheck/api"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	GetStatus(ip string) api.HealthStatus
}

//go:generate counterfeiter . HealthWatcher

type HealthWatcher interface {
	HealthState(ip string) api.HealthStatus
	HealthStateString(ip string) string
	Track(ip string)
	Untrack(ip string)
	Run(signal <-chan struct{})
	RunCheck(ip string)
}

type healthWatcher struct {
	checker       HealthChecker
	checkInterval time.Duration
	clock         clock.Clock
	workpoolSize  int

	checkWorkPool *workpool.WorkPool
	state         map[string]api.HealthStatus
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
		state:         map[string]api.HealthStatus{},
		stateMutex:    &sync.RWMutex{},
		logger:        logger,
	}
}

func (hw *healthWatcher) Track(ip string) {
	hw.checkWorkPool.Submit(func() {
		hw.RunCheck(ip)
	})
}

func (hw *healthWatcher) HealthStateString(ip string) string {
	return string(hw.HealthState(ip))
}

func (hw *healthWatcher) HealthState(ip string) api.HealthStatus {
	hw.stateMutex.RLock()
	health, found := hw.state[ip]
	hw.stateMutex.RUnlock()

	if !found {
		return StateUnchecked
	}
	return health
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

func (hw *healthWatcher) RunCheck(ip string) {
	isHealthy := hw.checker.GetStatus(ip)

	hw.stateMutex.Lock()

	wasHealthy, found := hw.state[ip]
	hw.state[ip] = isHealthy

	hw.stateMutex.Unlock()

	oldState := wasHealthy
	newState := isHealthy

	if !found {
		hw.logger.Debug("healthWatcher", "Initial state for IP <%s> is %s", ip, oldState)
	} else if oldState != newState {
		hw.logger.Debug("healthWatcher", "State for IP <%s> changed from %s to %s", ip, oldState, newState)
	}
}

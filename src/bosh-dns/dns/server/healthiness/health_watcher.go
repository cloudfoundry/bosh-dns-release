package healthiness

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	GetStatus(ip string) bool
}

type HealthWatcher interface {
	IsHealthy(ip string) bool
	Run(signal <-chan struct{})
}

type healthWatcher struct {
	checker       HealthChecker
	checkInterval time.Duration
	clock         clock.Clock

	checkWorkPool *workpool.WorkPool
	state         map[string]bool
	stateMutex    *sync.Mutex
}

func NewHealthWatcher(checker HealthChecker, clock clock.Clock, checkInterval time.Duration) *healthWatcher {
	wp, _ := workpool.NewWorkPool(1000)

	return &healthWatcher{
		checker:       checker,
		checkInterval: checkInterval,
		clock:         clock,

		checkWorkPool: wp,
		state:         map[string]bool{},
		stateMutex:    &sync.Mutex{},
	}
}

func (hw *healthWatcher) IsHealthy(ip string) bool {
	hw.stateMutex.Lock()
	defer hw.stateMutex.Unlock()

	if health, found := hw.state[ip]; found {
		return health
	}

	hw.state[ip] = false

	hw.checkWorkPool.Submit(func() {
		hw.runCheck(ip)
	})

	return true
}

func (hw *healthWatcher) Run(signal <-chan struct{}) {
	timer := hw.clock.NewTimer(hw.checkInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C():
			hw.stateMutex.Lock()
			for ip := range hw.state {
				// closing on ip, we need to ensure it's fixed within this context
				ip := ip
				hw.checkWorkPool.Submit(func() {
					hw.runCheck(ip)
				})
			}
			hw.stateMutex.Unlock()
		case <-signal:
			return
		}
		timer.Reset(hw.checkInterval)
	}
}

func (hw *healthWatcher) runCheck(ip string) {
	status := hw.checker.GetStatus(ip)

	hw.stateMutex.Lock()
	defer hw.stateMutex.Unlock()

	hw.state[ip] = status
}

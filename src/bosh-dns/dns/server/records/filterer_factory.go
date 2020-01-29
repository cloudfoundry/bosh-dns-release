package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/record"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
)

//go:generate counterfeiter . Filterer
type Filterer interface {
	Filter(crit criteria.MatchMaker, recs []record.Record) []record.Record
}

//go:generate counterfeiter . FiltererFactory
type FiltererFactory interface {
	NewHealthFilterer(healthChan chan record.Host, shouldTrack bool) Filterer
	NewQueryFilterer() Filterer
}

type healthFiltererFactory struct {
	healthWatcher healthiness.HealthWatcher
	synchronousCheckTimeout time.Duration
}

func (hff *healthFiltererFactory) NewHealthFilterer(healthChan chan record.Host, shouldTrack bool) Filterer {
	hf := NewHealthFilter(hff.NewQueryFilterer(), healthChan, hff.healthWatcher, shouldTrack, clock.NewClock(), hff.synchronousCheckTimeout, &sync.WaitGroup{})
	return &hf
}

func (hff *healthFiltererFactory) NewQueryFilterer() Filterer {
	return &QueryFilter{}
}

func NewHealthFiltererFactory(healthWatcher healthiness.HealthWatcher, synchronousCheckTimeout time.Duration) FiltererFactory {
	return &healthFiltererFactory{
		healthWatcher: healthWatcher,
		synchronousCheckTimeout: synchronousCheckTimeout,
	}
}

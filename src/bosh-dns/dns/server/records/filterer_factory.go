package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/record"
)

//go:generate counterfeiter . Filterer
type Filterer interface {
	Filter(crit criteria.MatchMaker, recs []record.Record) []record.Record
}

//go:generate counterfeiter . FiltererFactory
type FiltererFactory interface {
	NewFilterer(healthChan chan record.Host, shouldTrack bool) Filterer
}

type healthFiltererFactory struct {
	healthWatcher healthiness.HealthWatcher
}

func (hff *healthFiltererFactory) NewFilterer(healthChan chan record.Host, shouldTrack bool) Filterer {
	hf := NewHealthFilter(&QueryFilter{}, healthChan, hff.healthWatcher, shouldTrack)
	return &hf
}

func NewHealthFiltererFactory(healthWatcher healthiness.HealthWatcher) FiltererFactory {
	return &healthFiltererFactory{healthWatcher: healthWatcher}
}

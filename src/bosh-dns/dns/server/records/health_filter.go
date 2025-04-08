package records

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"

	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/record"
	"bosh-dns/healthcheck/api"
)

type healthFilter struct {
	nextFilter              Reducer
	health                  chan<- record.Host
	w                       healthWatcher
	wg                      *sync.WaitGroup
	shouldTrack             bool
	domain                  string //nolint:unused
	filterWorkPool          *workpool.WorkPool
	clock                   clock.Clock
	synchronousCheckTimeout time.Duration
}

type healthTracker interface { //nolint:unused
	MonitorRecordHealth(ip, fqdn string)
}

type healthWatcher interface {
	HealthState(ip string) api.HealthResult
	Track(ip string)
	RunCheck(ip string) api.HealthResult
}

func NewHealthFilter(nextFilter Reducer, health chan<- record.Host, w healthWatcher, shouldTrack bool, clock clock.Clock, synchronousCheckTimeout time.Duration, wg *sync.WaitGroup) healthFilter {
	wp, _ := workpool.NewWorkPool(1000) //nolint:errcheck
	return healthFilter{
		nextFilter:              nextFilter,
		health:                  health,
		w:                       w,
		wg:                      wg,
		shouldTrack:             shouldTrack,
		filterWorkPool:          wp,
		clock:                   clock,
		synchronousCheckTimeout: synchronousCheckTimeout,
	}
}

func (q *healthFilter) Filter(mm criteria.MatchMaker, recs []record.Record) []record.Record {
	crit, ok := mm.(criteria.Criteria)
	if !ok {
		crit, _ = criteria.NewCriteria("", []string{}) //nolint:errcheck
	}
	records := q.nextFilter.Filter(crit, recs)

	healthStrategy := "0"
	if len(crit["s"]) > 0 {
		healthStrategy = crit["s"][0]
	}

	skipTracking := false //nolint:staticcheck
	if healthStrategy == "0" && len(records) == 1 {
		// if there's only 1 target the smart strategy will always return it, healthy or not
		// there's no value in tracking the health for this fqdn
		skipTracking = true
	}

	if q.shouldTrack && !skipTracking {
		q.processRecords(crit, records)
	}

	healthyRecords, unhealthyRecords, maybeHealthyRecords := q.sortRecords(records, crit["g"])

	switch healthStrategy {
	case "1": // unhealthy ones
		return unhealthyRecords
	case "3": // healthy
		return healthyRecords
	case "4": // all
		return records
	default: // smart strategy
		if len(maybeHealthyRecords) == 0 {
			return records
		}

		return maybeHealthyRecords
	}
}

func (q *healthFilter) processRecords(criteria criteria.Criteria, records []record.Record) {
	usedWaitGroup := false

	for _, r := range records {
		if fqdn, ok := criteria["fqdn"]; ok {
			q.health <- record.Host{IP: r.IP, FQDN: fqdn[0]}

			if len(criteria["y"]) > 0 {
				if q.synchronousHealthCheck(criteria["y"][0], r.IP) {
					usedWaitGroup = true
				}
			}
		}
	}

	if usedWaitGroup {
		q.waitForWaitGroupOrTimeout()
	}
}

func (q *healthFilter) waitForWaitGroupOrTimeout() {
	timeout := q.clock.After(q.synchronousCheckTimeout)
	success := make(chan struct{})

	go func() {
		q.wg.Wait()
		close(success)
	}()

	for {
		select {
		case <-timeout:
			return
		case <-success:
			return
		}
	}
}

func (q *healthFilter) sortRecords(records []record.Record, queriedGroupIDs []string) (healthyRecords, unhealthyRecords, maybeHealthyRecords []record.Record) {
	var unknownRecords, uncheckedRecords []record.Record

	for _, r := range records {
		switch q.interpretHealthState(r.IP, queriedGroupIDs) {
		case api.StatusRunning:
			healthyRecords = append(healthyRecords, r)
		case api.StatusFailing:
			unhealthyRecords = append(unhealthyRecords, r)
		case healthiness.StateUnknown:
			unknownRecords = append(unknownRecords, r) //nolint:staticcheck
		case healthiness.StateUnchecked:
			uncheckedRecords = append(uncheckedRecords, r)
		}
	}

	maybeHealthyRecords = append(healthyRecords, uncheckedRecords...)

	return healthyRecords, unhealthyRecords, maybeHealthyRecords
}

func (q *healthFilter) interpretHealthState(ip string, queriedGroupIDs []string) api.HealthStatus {
	queriedHealthState := q.w.HealthState(ip)
	healthState := queriedHealthState.State

	for _, groupID := range queriedGroupIDs {
		if groupState, ok := queriedHealthState.GroupState[groupID]; ok {
			if groupState == api.StatusFailing {
				return api.StatusFailing
			}
			healthState = groupState
		}
	}

	return healthState
}

func (q *healthFilter) synchronousHealthCheck(strategy, ip string) bool {
	usedWaitGroup := false
	switch strategy {
	case "0":
	case "1":
		if q.w.HealthState(ip).State == healthiness.StateUnchecked {
			q.wg.Add(1)
			q.filterWorkPool.Submit(func() {
				defer q.wg.Done()
				q.w.RunCheck(ip)
			})
		}
		usedWaitGroup = true
	case "2":
		// q.runCheck(ip) to be implemented in a future story
	}

	return usedWaitGroup
}

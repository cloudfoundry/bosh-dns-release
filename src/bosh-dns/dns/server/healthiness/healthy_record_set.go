package healthiness

import (
	"bosh-dns/dns/server/healthiness/internal"
	"bosh-dns/dns/server/records"
	"sync"
)

//go:generate counterfeiter . RecordSetRepo

type RecordSetRepo interface {
	Get() (records.RecordSet, error)
	Subscribe() <-chan bool
}

type HealthyRecordSet struct {
	healthWatcher HealthWatcher

	recordSet      records.RecordSet
	recordSetMutex *sync.RWMutex

	trackedDomains *internal.PriorityLimitedTranscript

	trackedIPs      map[string]map[string]struct{}
	trackedIPsMutex *sync.Mutex
}

func NewHealthyRecordSet(
	recordSetRepo RecordSetRepo,
	healthWatcher HealthWatcher,
	maximumTrackedDomains uint,
	shutdownChan chan struct{},
) *HealthyRecordSet {
	subscriptionChan := recordSetRepo.Subscribe()
	recordSet, _ := recordSetRepo.Get()

	hrs := &HealthyRecordSet{
		healthWatcher: healthWatcher,

		recordSet:      recordSet,
		recordSetMutex: &sync.RWMutex{},

		trackedDomains: internal.NewPriorityLimitedTranscript(maximumTrackedDomains),

		trackedIPs:      map[string]map[string]struct{}{},
		trackedIPsMutex: &sync.Mutex{},
	}

	go func() {
		for {
			select {
			case <-shutdownChan:
				return
			case ok := <-subscriptionChan:
				if !ok {
					return
				}
				recordSet, err := recordSetRepo.Get()
				if err != nil {
					continue
				}

				hrs.recordSetMutex.Lock()
				hrs.recordSet = recordSet
				hrs.recordSetMutex.Unlock()

				hrs.refreshTrackedIPs()
			}
		}
	}()

	return hrs
}

func (hrs *HealthyRecordSet) refreshTrackedIPs() {
	hrs.recordSetMutex.RLock()
	recordSet := hrs.recordSet
	hrs.recordSetMutex.RUnlock()

	newTrackedIPs := map[string]map[string]struct{}{}
	hrs.trackedIPsMutex.Lock()
	defer hrs.trackedIPsMutex.Unlock()
	for _, domain := range hrs.trackedDomains.Registry() {
		ips, err := recordSet.Resolve(domain)
		if err != nil {
			continue
		}

		for _, ip := range ips {
			if _, ok := newTrackedIPs[ip]; !ok {
				newTrackedIPs[ip] = map[string]struct{}{}
			}
			newTrackedIPs[ip][domain] = struct{}{}

			if _, found := hrs.trackedIPs[ip]; found {
				delete(hrs.trackedIPs, ip)
			} else {
				hrs.healthWatcher.IsHealthy(ip)
			}
		}
	}
	for oldIP := range hrs.trackedIPs {
		hrs.healthWatcher.Untrack(oldIP)
	}
	hrs.trackedIPs = newTrackedIPs
}

func (hrs *HealthyRecordSet) untrackDomain(removedDomain string) {
	hrs.trackedIPsMutex.Lock()
	defer hrs.trackedIPsMutex.Unlock()

	for ip, domains := range hrs.trackedIPs {
		if _, ok := domains[removedDomain]; ok {
			delete(domains, removedDomain)
			if len(domains) == 0 {
				hrs.healthWatcher.Untrack(ip)
			}
		}
	}
}

func (hrs *HealthyRecordSet) Resolve(fqdn string) ([]string, bool, error) {
	hrs.recordSetMutex.RLock()
	recordSet := hrs.recordSet
	hrs.recordSetMutex.RUnlock()

	ips, err := recordSet.Resolve(fqdn)
	if err != nil {
		return nil, false, err
	}

	if removed := hrs.trackedDomains.Touch(fqdn); removed != "" {
		hrs.untrackDomain(removed)
	}

	healthyIPs := []string{}
	unhealthyIPs := []string{}

	for _, ip := range ips {
		hrs.trackedIPsMutex.Lock()
		hrs.trackedIPs[ip] = map[string]struct{}{}
		if _, ok := hrs.trackedIPs[ip]; !ok {
			hrs.trackedIPs[ip] = map[string]struct{}{}
		}
		hrs.trackedIPs[ip][fqdn] = struct{}{}
		hrs.trackedIPsMutex.Unlock()

		if hrs.healthWatcher.IsHealthy(ip) {
			healthyIPs = append(healthyIPs, ip)
		} else {
			unhealthyIPs = append(unhealthyIPs, ip)
		}
	}

	if len(healthyIPs) == 0 {
		return unhealthyIPs, false, nil
	}

	return healthyIPs, true, nil
}

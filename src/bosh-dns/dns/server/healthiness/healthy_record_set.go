package healthiness

import (
	"bosh-dns/dns/server/healthiness/internal"
	"sync"
)

//go:generate counterfeiter . RecordSet

type RecordSet interface {
	Resolve(domain string) ([]string, error)
	Subscribe() <-chan bool
}

type HealthyRecordSet struct {
	healthWatcher HealthWatcher

	recordSet RecordSet

	trackedDomains *internal.PriorityLimitedTranscript

	trackedIPs      map[string]map[string]struct{}
	trackedIPsMutex *sync.Mutex
}

func NewHealthyRecordSet(
	recordSet RecordSet,
	healthWatcher HealthWatcher,
	maximumTrackedDomains uint,
	shutdownChan chan struct{},
) *HealthyRecordSet {
	subscriptionChan := recordSet.Subscribe()

	hrs := &HealthyRecordSet{
		healthWatcher: healthWatcher,

		recordSet: recordSet,

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
				hrs.refreshTrackedIPs()
			}
		}
	}()

	return hrs
}

func (hrs *HealthyRecordSet) refreshTrackedIPs() {
	newTrackedIPs := map[string]map[string]struct{}{}
	hrs.trackedIPsMutex.Lock()
	defer hrs.trackedIPsMutex.Unlock()
	for _, domain := range hrs.trackedDomains.Registry() {
		ips, err := hrs.recordSet.Resolve(domain)
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

func (hrs *HealthyRecordSet) Resolve(fqdn string) ([]string, error) {
	ips, err := hrs.recordSet.Resolve(fqdn)
	if err != nil {
		return nil, err
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
		return unhealthyIPs, nil
	}

	return healthyIPs, nil
}

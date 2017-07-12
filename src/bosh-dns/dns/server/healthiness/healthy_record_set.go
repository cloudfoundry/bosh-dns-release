package healthiness

import (
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

	trackedDomains      map[string]struct{}
	trackedDomainsMutex *sync.RWMutex

	trackedIPs      map[string]struct{}
	trackedIPsMutex *sync.Mutex
}

func NewHealthyRecordSet(recordSetRepo RecordSetRepo, healthWatcher HealthWatcher, shutdownChan chan struct{}) *HealthyRecordSet {
	subscriptionChan := recordSetRepo.Subscribe()
	recordSet, _ := recordSetRepo.Get()

	hrs := &HealthyRecordSet{
		healthWatcher: healthWatcher,

		recordSet:      recordSet,
		recordSetMutex: &sync.RWMutex{},

		trackedDomains:      map[string]struct{}{},
		trackedDomainsMutex: &sync.RWMutex{},

		trackedIPs:      map[string]struct{}{},
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

				newTrackedIPs := map[string]struct{}{}
				hrs.trackedDomainsMutex.RLock()
				hrs.trackedIPsMutex.Lock()
				for domain := range hrs.trackedDomains {
					ips, err := recordSet.Resolve(domain)
					if err != nil {
						continue
					}

					for _, ip := range ips {
						newTrackedIPs[ip] = struct{}{}

						if _, found := hrs.trackedIPs[ip]; found {
							delete(hrs.trackedIPs, ip)
						} else {
							healthWatcher.IsHealthy(ip)
						}
					}
				}
				hrs.trackedDomainsMutex.RUnlock()

				for oldIP := range hrs.trackedIPs {
					hrs.healthWatcher.Untrack(oldIP)
				}

				hrs.trackedIPs = newTrackedIPs
				hrs.trackedIPsMutex.Unlock()
			}
		}
	}()

	return hrs
}

func (hrs *HealthyRecordSet) Resolve(fqdn string) ([]string, error) {
	hrs.recordSetMutex.RLock()
	recordSet := hrs.recordSet
	hrs.recordSetMutex.RUnlock()

	ips, err := recordSet.Resolve(fqdn)
	if err != nil {
		return nil, err
	}

	hrs.trackedDomainsMutex.Lock()
	hrs.trackedDomains[fqdn] = struct{}{}
	hrs.trackedDomainsMutex.Unlock()

	healthyIPs := []string{}
	unhealthyIPs := []string{}

	for _, ip := range ips {
		hrs.trackedIPsMutex.Lock()
		hrs.trackedIPs[ip] = struct{}{}
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

package healthiness

import "bosh-dns/dns/server/records"

//go:generate counterfeiter . RecordSetRepo

type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

type HealthyRecordSet struct {
	recordSetRepo RecordSetRepo
	healthWatcher HealthWatcher
}

func NewHealthyRecordSet(recordSetRepo RecordSetRepo, healthWatcher HealthWatcher) *HealthyRecordSet {
	return &HealthyRecordSet{
		recordSetRepo: recordSetRepo,
		healthWatcher: healthWatcher,
	}
}

func (hrs *HealthyRecordSet) Resolve(fqdn string) ([]string, error) {
	recordSet, err := hrs.recordSetRepo.Get()
	if err != nil {
		return nil, err
	}

	ips, err := recordSet.Resolve(fqdn)
	if err != nil {
		return nil, err
	}

	healthyIPs := []string{}
	unhealthyIPs := []string{}

	for _, ip := range ips {
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

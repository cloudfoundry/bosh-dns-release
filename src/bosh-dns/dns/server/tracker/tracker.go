package tracker

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
	"sync"
)

type Tracker struct {
	trackedDomains  limitedTranscript
	h               healther
	trackedIPs      map[string]map[string]struct{}
	trackedIPsMutex *sync.Mutex
	qf              query
}

//go:generate counterfeiter -o ./fakes/limited_transcript.go --fake-name LimitedTranscript . limitedTranscript
type limitedTranscript interface {
	Touch(string) string
	Registry() []string
}

//go:generate counterfeiter -o ./fakes/healther.go --fake-name Healther . healther
type healther interface {
	Track(ip string)
	Untrack(ip string)
	IsHealthy(ip string) bool
}

//go:generate counterfeiter -o ./fakes/query.go --fake-name Query . query
type query interface {
	Filter(criteria.Criteria, []record.Record) []record.Record
}

func Start(shutdown chan struct{}, subscription <-chan []record.Record, healthMonitor <-chan record.Host, trackedDomains limitedTranscript, healther healther, qf query) {
	t := &Tracker{
		trackedDomains:  trackedDomains,
		h:               healther,
		qf:              qf,
		trackedIPs:      map[string]map[string]struct{}{},
		trackedIPsMutex: &sync.Mutex{},
	}

	go func() {
		for {
			select {
			case <-shutdown:
				return
			case host := <-healthMonitor:
				t.monitor(host.IP, host.FQDN)
			case recs := <-subscription:
				t.refresh(recs)
			}
		}
	}()
}

func (t *Tracker) monitor(ip, fqdn string) {
	t.trackedIPsMutex.Lock()

	if remove := t.trackedDomains.Touch(fqdn); remove != "" {
		for ip, domains := range t.trackedIPs {
			if _, ok := domains[remove]; ok {
				delete(domains, remove)
				if len(domains) == 0 {
					t.h.Untrack(ip)
				}
			}
		}
	}

	t.trackedIPs[ip] = map[string]struct{}{}
	t.trackedIPs[ip][fqdn] = struct{}{}
	t.trackedIPsMutex.Unlock()

	t.h.Track(ip)
}

func (t *Tracker) refresh(recs []record.Record) {
	newTrackedIPs := map[string]map[string]struct{}{}
	t.trackedIPsMutex.Lock()
	defer t.trackedIPsMutex.Unlock()
	domains := []string{}
	for _, rec := range recs {
		domains = append(domains, rec.Domain)
	}

	for _, domain := range t.trackedDomains.Registry() {
		crit, err := criteria.NewCriteria(domain, domains)
		if err != nil {
			continue
		}

		records := t.qf.Filter(crit, recs)
		ips := make([]string, len(records))
		for i, rec := range records {
			ips[i] = rec.IP
		}

		for _, ip := range ips {
			if _, ok := newTrackedIPs[ip]; !ok {
				newTrackedIPs[ip] = map[string]struct{}{}
			}
			newTrackedIPs[ip][domain] = struct{}{}

			if _, found := t.trackedIPs[ip]; found {
				delete(t.trackedIPs, ip)
			} else {
				newTrackedIPs[ip] = map[string]struct{}{}
				newTrackedIPs[ip][domain] = struct{}{}

				t.h.Track(ip)
			}
		}
	}

	for oldIP := range t.trackedIPs {
		t.h.Untrack(oldIP)
	}

	t.trackedIPs = newTrackedIPs
}

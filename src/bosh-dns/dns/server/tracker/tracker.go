package tracker

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
	"sync"

	"github.com/cloudfoundry/bosh-utils/logger"
)

type Tracker struct {
	trackedDomains  limitedTranscript
	h               healther
	trackedIPs      map[string]map[string]struct{}
	trackedIPsMutex *sync.Mutex
	qf              query
	logger          logger.Logger
}

//counterfeiter:generate -o ./fakes/limited_transcript.go --fake-name LimitedTranscript . limitedTranscript
type limitedTranscript interface {
	Touch(string) string
	Registry() []string
}

//counterfeiter:generate -o ./fakes/healther.go --fake-name Healther . healther
type healther interface {
	Track(ip string)
	Untrack(ip string)
}

//counterfeiter:generate -o ./fakes/query.go --fake-name Query . query
type query interface {
	Filter(criteria.MatchMaker, []record.Record) []record.Record
}

func Start(shutdown chan struct{}, subscription <-chan []record.Record, healthMonitor <-chan record.Host, trackedDomains limitedTranscript, healther healther, qf query, logger logger.Logger) {
	t := &Tracker{
		trackedDomains:  trackedDomains,
		h:               healther,
		qf:              qf,
		trackedIPs:      map[string]map[string]struct{}{},
		trackedIPsMutex: &sync.Mutex{},
		logger:          logger,
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
		t.logger.Debug("Tracker", "remove %s from recent domains", fqdn)
		for ip, domains := range t.trackedIPs {
			if _, ok := domains[remove]; ok {
				delete(domains, remove)
				if len(domains) == 0 {
					t.logger.Debug("Tracker", "remove %s from tracked IPs - %s was last domain", ip, fqdn)
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

func (t *Tracker) refresh(newRecords []record.Record) {
	newTrackedIPs := map[string]map[string]struct{}{}
	t.trackedIPsMutex.Lock()
	defer t.trackedIPsMutex.Unlock()
	recordDomains := []string{}
	for _, rec := range newRecords {
		recordDomains = append(recordDomains, rec.Domain)
	}

	monitoredDomains := t.trackedDomains.Registry()
	for _, domain := range monitoredDomains {
		crit, err := criteria.NewCriteria(domain, recordDomains)
		if err != nil {
			t.logger.Warn("Tracker", "Error creating filter criteria for %s", domain)
			continue
		}

		filteredRecords := t.qf.Filter(crit, newRecords)

		for _, rec := range filteredRecords {
			ip := rec.IP
			firstOccurrence := false
			if _, ok := newTrackedIPs[ip]; !ok {
				firstOccurrence = true
				newTrackedIPs[ip] = map[string]struct{}{}
			}
			newTrackedIPs[ip][domain] = struct{}{}

			previouslyTracked := false
			if _, found := t.trackedIPs[ip]; found {
				delete(t.trackedIPs, ip)
				previouslyTracked = true
			}

			if firstOccurrence && !previouslyTracked {
				t.logger.Debug("Tracker", "Found new IP %s for %s in refreshed records", ip, domain)
				t.h.Track(ip)
			} else {
				t.logger.Debug("Tracker", "Found tracked IP %s for %s in refreshed records", ip, domain)
			}
		}
	}

	for oldIP := range t.trackedIPs {
		t.logger.Debug("Tracker", "IP %s not referenced by refreshed domains", oldIP)
		t.h.Untrack(oldIP)
	}

	t.trackedIPs = newTrackedIPs
}

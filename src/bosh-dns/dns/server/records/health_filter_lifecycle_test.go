package records_test

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
	"bosh-dns/healthcheck/api"

	"code.cloudfoundry.org/clock"
)

func poolWorkers() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "code.cloudfoundry.org/workpool.worker(")
}

func TestHealthFilterReleasesCompletedWorkers(t *testing.T) {
	before := poolWorkers()
	for range 20 {
		watcher := &healthinessfakes.FakeHealthWatcher{}
		watcher.HealthStateReturns(api.HealthResult{State: healthiness.StateUnchecked})
		reducer := &recordsfakes.FakeReducer{}
		recs := []record.Record{{IP: "127.0.0.1"}}
		reducer.FilterReturns(recs)
		filter := records.NewHealthFilter(reducer, make(chan record.Host, 1), watcher, true, clock.NewClock(), time.Second, &sync.WaitGroup{})
		filter.Filter(criteria.Criteria{"s": {"3"}, "y": {"1"}, "fqdn": {"test.bosh."}}, recs)
	}
	deadline := time.Now().Add(time.Second)
	for poolWorkers() > before && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if after := poolWorkers(); after > before {
		t.Fatalf("workpool workers remain after checks completed: before=%d after=%d", before, after)
	}
}

func TestHealthFilterRunsChecksForEachCriteria(t *testing.T) {
	watcher := &healthinessfakes.FakeHealthWatcher{}
	watcher.HealthStateReturns(api.HealthResult{State: healthiness.StateUnchecked})
	reducer := &recordsfakes.FakeReducer{}
	reducer.FilterStub = func(_ criteria.MatchMaker, recs []record.Record) []record.Record { return recs }
	filter := records.NewHealthFilter(reducer, make(chan record.Host, 2), watcher, true, clock.NewClock(), 5*time.Second, &sync.WaitGroup{})

	start := time.Now()
	filter.Filter(criteria.Criteria{"s": {"3"}, "y": {"1"}, "fqdn": {"first.bosh."}}, []record.Record{{IP: "127.0.0.1"}})
	filter.Filter(criteria.Criteria{"s": {"3"}, "y": {"1"}, "fqdn": {"second.bosh."}}, []record.Record{{IP: "127.0.0.2"}})

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("synchronous checks waited for the timeout: %s", elapsed)
	}
	if count := watcher.RunCheckCallCount(); count != 2 {
		t.Fatalf("expected 2 synchronous checks, got %d", count)
	}
}

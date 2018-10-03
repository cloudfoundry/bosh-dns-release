package healthiness

import "bosh-dns/healthcheck/api"

type nopHealthWatcher struct{}

func NewNopHealthWatcher() *nopHealthWatcher {
	return &nopHealthWatcher{}
}

func (hw *nopHealthWatcher) Track(ip string) {
}

func (hw *nopHealthWatcher) HealthState(ip string) api.HealthStatus {
	return api.StatusRunning
}

func (hw *nopHealthWatcher) HealthStateString(ip string) string {
	return string(api.StatusRunning)
}

func (hw *nopHealthWatcher) Untrack(ip string) {}

func (hw *nopHealthWatcher) Run(signal <-chan struct{}) {
	<-signal
}

func (hw *nopHealthWatcher) RunCheck(ip string) {}

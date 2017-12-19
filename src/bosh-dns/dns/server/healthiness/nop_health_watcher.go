package healthiness

type nopHealthWatcher struct{}

func NewNopHealthWatcher() *nopHealthWatcher {
	return &nopHealthWatcher{}
}

func (hw *nopHealthWatcher) IsHealthy(ip string) bool {
	return true
}

func (hw *nopHealthWatcher) Track(ip string) {
}

func (hw *nopHealthWatcher) HealthState(ip string) string {
	return StateHealthy
}

func (hw *nopHealthWatcher) Untrack(ip string) {}

func (hw *nopHealthWatcher) Run(signal <-chan struct{}) {
	<-signal
}

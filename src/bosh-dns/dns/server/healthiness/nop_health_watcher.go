package healthiness

type nopHealthWatcher struct{}

func NewNopHealthWatcher() *nopHealthWatcher {
	return &nopHealthWatcher{}
}

func (hw *nopHealthWatcher) Track(ip string) {
}

func (hw *nopHealthWatcher) HealthState(ip string) HealthState {
	return StateHealthy
}

func (hw *nopHealthWatcher) HealthStateString(ip string) string {
	return string(StateHealthy)
}

func (hw *nopHealthWatcher) Untrack(ip string) {}

func (hw *nopHealthWatcher) Run(signal <-chan struct{}) {
	<-signal
}

func (hw *nopHealthWatcher) RunCheck(ip string) {}

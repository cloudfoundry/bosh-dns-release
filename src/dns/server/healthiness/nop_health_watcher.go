package healthiness

type nopHealthWatcher struct{}

func NewNopHealthWatcher() *nopHealthWatcher {
	return &nopHealthWatcher{}
}

func (hw *nopHealthWatcher) IsHealthy(ip string) bool {
	return true
}

func (hw *nopHealthWatcher) Run(signal <-chan struct{}) {
	<-signal
}

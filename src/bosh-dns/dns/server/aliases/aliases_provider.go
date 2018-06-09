package aliases

import (
	"sync"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type AliasesProvider interface {
	Resolutions(maybeAlias string) []string
	AliasHosts() []string
}

type AutoRefreshAliasesProvider struct {
	checkInterval time.Duration
	config        Config
	configs       map[string]Config
	logger        boshlog.Logger
	loader        NamedConfigLoader
	glob          string
	fs            system.FileSystem
	configMutex   sync.RWMutex
}

func NewAutoRefreshAliasesProvider(
	logger boshlog.Logger,
	glob string,
	fs system.FileSystem,
	checkInterval time.Duration,
) AliasesProvider {
	provider := &AutoRefreshAliasesProvider{
		logger:        logger,
		loader:        NewFSLoader(fs),
		glob:          glob,
		fs:            fs,
		checkInterval: checkInterval,
		configs:       make(map[string]Config),
	}
	go provider.startAutoRefresh()
	return provider
}

func (a *AutoRefreshAliasesProvider) startAutoRefresh() {
	fileTrigger := NewFileEventTrigger(a.logger, a.fs, a.glob, a.checkInterval)
	subscriptionChan := fileTrigger.Subscribe()

	go fileTrigger.Start()

	for {
		select {
		case event := <-subscriptionChan:
			a.logger.Info("SUBSCRIBER_EVENT_HAPPENED", "%s", event.String())
			switch event.Type {
			case Updated:
				a.Updated(&event)
				break
			case Added:
				a.Added(&event)
				break
			case Deleted:
				a.Deleted(&event)
				break
			}
		}
	}

}

func (a *AutoRefreshAliasesProvider) Updated(event *FileEvent) {
	cfg, err := a.loader.Load(event.File)
	if err != nil {
		a.logger.Error("LOAD_FAILED", "%s", event.String())
	}
	a.configs[event.File] = cfg
	a.refresh()
}

func (a *AutoRefreshAliasesProvider) Deleted(event *FileEvent) {
	delete(a.configs, event.File)
	a.refresh()
}

func (a *AutoRefreshAliasesProvider) Added(event *FileEvent) {
	cfg, err := a.loader.Load(event.File)
	if err != nil {
		a.logger.Error("LOAD_FAILED", "%s", event.String()) // TODO error handling
	}
	a.configs[event.File] = cfg
	a.refresh()
}

func (a *AutoRefreshAliasesProvider) refresh() {
	config := NewConfig()
	for _, cfg := range a.configs {
		config = config.Merge(cfg)
	}
	canonicalAliases, err := config.ReducedForm()
	if err != nil {
		a.logger.Error("REDUCED_FORM_FAILED", "%s", err)
	} else {
		a.configMutex.Lock()
		defer a.configMutex.Unlock()
		a.config = canonicalAliases
	}
}

func (a *AutoRefreshAliasesProvider) Resolutions(maybeAlias string) []string {
	a.configMutex.RLock()
	defer a.configMutex.RUnlock()
	return a.config.Resolutions(maybeAlias)
}

func (a *AutoRefreshAliasesProvider) AliasHosts() []string {
	a.configMutex.RLock()
	defer a.configMutex.RUnlock()
	return a.config.AliasHosts()
}

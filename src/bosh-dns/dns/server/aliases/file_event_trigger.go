package aliases

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type EventType int

const (
	Added   EventType = iota
	Updated EventType = iota
	Deleted EventType = iota
)

type FileEvent struct {
	File string
	Type EventType
}

func (f *FileEvent) String() string {
	return fmt.Sprintf("%s,%d", f.File, f.Type)
}

//go:generate counterfeiter . FileEventTrigger
type FileEventTrigger struct {
	fs               system.FileSystem
	glob             string
	cacheStats       map[string]os.FileInfo
	logger           boshlog.Logger
	subscribers      []chan FileEvent
	checkInterval    time.Duration
	subscribersMutex sync.RWMutex
	notifyMutex      sync.RWMutex
}

func NewFileEventTrigger(
	logger boshlog.Logger,
	fs system.FileSystem,
	glob string,
	checkInterval time.Duration,
) *FileEventTrigger {
	return &FileEventTrigger{
		logger:        logger,
		fs:            fs,
		glob:          glob,
		checkInterval: checkInterval,
		cacheStats:    make(map[string]os.FileInfo),
		subscribers:   []chan FileEvent{},
	}
}

func (a *FileEventTrigger) Start() {
	a.roundCheck()

	clock := clock.NewClock()
	timer := clock.NewTimer(a.checkInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C():
			a.logger.Info("FILE_CHECK_TIME_INTERVAL_REACHED", "%s", a.glob)
			a.roundCheck()
			timer.Reset(a.checkInterval)
			continue
		}
	}
}

func (a *FileEventTrigger) roundCheck() {
	files, err := a.fs.Glob(a.glob)
	if err != nil {
		// return Config{}, bosherr.WrapError(err, "glob pattern failed to compute")
	}
	filesInDisk := map[string]bool{}
	for _, file := range files {
		if a.fs.FileExists(file) {
			filesInDisk[file] = true
			a.logger.Info("CHECK_FILE", "%s", file)
			newStat, err := a.fs.StatWithOpts(file, system.StatOpts{Quiet: true})
			if err != nil {
				a.logger.Warn("CHECK_FILE_FAILED", "f:%s err: %s", file, err)
				break
			}
			cacheStat := a.cacheStats[file]
			if cacheStat != nil {
				if !reflect.DeepEqual(cacheStat, newStat) {
					a.cacheStats[file] = newStat
					a.notify(file, Updated)
				}
			} else {
				a.cacheStats[file] = newStat
				a.notify(file, Added)
			}
		}
	}

	toRemove := []string{}
	for k := range a.cacheStats {
		if !filesInDisk[k] {
			toRemove = append(toRemove, k)
		}
	}

	for _, item := range toRemove {
		delete(a.cacheStats, item)
		a.notify(item, Deleted)
	}
}

func (a *FileEventTrigger) notify(file string, eventType EventType) {
	a.notifyMutex.Lock()
	defer a.notifyMutex.Unlock()

	for _, subscriber := range a.subscribers {
		fileEvent := FileEvent{
			File: file,
			Type: eventType,
		}
		a.logger.Info("NOTIFY_SUBSCRIBER", "%s", fileEvent.String())
		subscriber <- fileEvent
	}
}

func (a *FileEventTrigger) Subscribe() <-chan FileEvent {
	a.subscribersMutex.Lock()
	defer a.subscribersMutex.Unlock()
	c := make(chan FileEvent)
	a.subscribers = append(a.subscribers, c)
	return c
}

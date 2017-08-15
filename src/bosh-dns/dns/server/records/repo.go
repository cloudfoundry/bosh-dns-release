package records

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"

	"os"
	"reflect"
	"sync"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

const logTag string = "RecordsRepo"

type RecordSetProvider interface {
	Get() (RecordSet, error)
	Subscribe() <-chan bool
}

type autoUpdatingRepo struct {
	// updateImmediately chan struct{}
	recordsFilePath string
	fileSystem      system.FileSystem
	clock           clock.Clock
	logger          logger.Logger
	rwlock          *sync.RWMutex
	cacheStat       os.FileInfo
	cache           *RecordSet
	cacheErr        error

	subscribers []chan bool
}

func NewRepo(recordsFilePath string, fileSys system.FileSystem, clock clock.Clock, logger logger.Logger, shutdownChan chan struct{}) RecordSetProvider {
	repo := &autoUpdatingRepo{
		recordsFilePath: recordsFilePath,
		fileSystem:      fileSys,
		clock:           clock,
		logger:          logger,
		rwlock:          &sync.RWMutex{},

		subscribers: []chan bool{},
	}

	_, records, err := repo.needNewFromDisk()
	repo.atomicallyUpdateCache(&records, err)

	if repo.cacheErr != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to open records file at: %s", recordsFilePath))
	}

	go func() {
		for {
			select {
			case <-shutdownChan:
				break
			default:
				clock.Sleep(time.Second)

				newData, data, err := repo.needNewFromDisk()
				if newData && err == nil {
					repo.atomicallyUpdateCache(&data, err)
					for _, c := range repo.subscribers {
						c <- true
					}
				}
			}
		}
	}()

	return repo
}

func (r *autoUpdatingRepo) Subscribe() <-chan bool {
	c := make(chan bool)
	r.subscribers = append(r.subscribers, c)
	return c
}

func (r *autoUpdatingRepo) needNewFromDisk() (needsNew bool, set RecordSet, err error) {
	needsNew = false

	var newStat os.FileInfo
	newStat, err = r.fileSystem.Stat(r.recordsFilePath)
	if err != nil {
		err = bosherr.Errorf("Error stating records file '%s': %s", r.recordsFilePath, err.Error())
		return
	}

	if reflect.DeepEqual(r.cacheStat, newStat) {
		return
	} else {
		needsNew = true
	}

	var buf []byte
	buf, err = r.fileSystem.ReadFile(r.recordsFilePath)
	if err != nil {
		return
	}

	r.cacheStat = newStat

	set, err = CreateFromJSON(buf, r.logger)

	return
}

func (r *autoUpdatingRepo) Get() (RecordSet, error) {
	setPtr, err := r.atomicallyFetchCache()
	if setPtr == nil || err != nil {
		return RecordSet{}, err
	}

	return *setPtr, nil
}

func (r *autoUpdatingRepo) atomicallyUpdateCache(set *RecordSet, err error) {
	r.rwlock.Lock()
	r.cache = set
	r.cacheErr = err
	r.rwlock.Unlock()
}

func (r *autoUpdatingRepo) atomicallyFetchCache() (set *RecordSet, err error) {
	r.rwlock.RLock()
	set = r.cache
	err = r.cacheErr
	r.rwlock.RUnlock()
	return
}

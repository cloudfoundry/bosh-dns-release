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

//counterfeiter:generate . FileReader

type FileReader interface {
	Get() ([]byte, error)
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
	cache           []byte
	cacheErr        error

	subscribers []chan bool
}

func NewFileReader(recordsFilePath string, fileSys system.FileSystem, clock clock.Clock, logger logger.Logger, shutdownChan chan struct{}) FileReader {
	repo := &autoUpdatingRepo{
		recordsFilePath: recordsFilePath,
		fileSystem:      fileSys,
		clock:           clock,
		logger:          logger,
		rwlock:          &sync.RWMutex{},

		subscribers: []chan bool{},
	}

	_, fileContents, err := repo.needNewFromDisk()
	repo.atomicallyUpdateCache(fileContents, err)

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
					repo.atomicallyUpdateCache(data, err)
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

func (r *autoUpdatingRepo) needNewFromDisk() (bool, []byte, error) {
	newStat, err := r.fileSystem.StatWithOpts(r.recordsFilePath, system.StatOpts{Quiet: true})
	if err != nil {
		return false, nil, bosherr.Errorf("Error stating records file '%s': %s", r.recordsFilePath, err.Error())
	}

	if reflect.DeepEqual(r.cacheStat, newStat) {
		return false, nil, nil
	}

	var buf []byte
	buf, err = r.fileSystem.ReadFile(r.recordsFilePath)
	if err != nil {
		return true, nil, err
	}

	r.cacheStat = newStat

	return true, buf, nil
}

func (r *autoUpdatingRepo) Get() ([]byte, error) {
	setPtr, err := r.atomicallyFetchCache()
	if setPtr == nil || err != nil {
		return nil, err
	}

	return setPtr, nil
}

func (r *autoUpdatingRepo) atomicallyUpdateCache(cachedFileContents []byte, err error) {
	r.rwlock.Lock()
	r.cache = cachedFileContents
	r.cacheErr = err
	r.rwlock.Unlock()
}

func (r *autoUpdatingRepo) atomicallyFetchCache() ([]byte, error) {
	r.rwlock.RLock()
	set := r.cache
	err := r.cacheErr
	r.rwlock.RUnlock()
	return set, err
}

package records

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"os"
	"reflect"
	"sync"
)

const logTag string = "RecordsRepo"

type Repo struct {
	fileSystem           system.FileSystem
	recordsFilePath      string
	cachedRecordSetError error
	cachedRecordSet      RecordSet
	stat                 os.FileInfo
	mutex                *sync.Mutex
	logger               logger.Logger
}

func NewRepo(recordsFilePath string, fileSys system.FileSystem, logger logger.Logger) *Repo {
	repo := Repo{
		recordsFilePath: recordsFilePath,
		fileSystem:      fileSys,
		mutex:           &sync.Mutex{},
		logger:          logger,
	}

	repo.cachedRecordSet, repo.cachedRecordSetError = repo.createFromFileSystem()
	if repo.cachedRecordSetError != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to open records file at: %s", recordsFilePath))
	}
	return &repo
}

func (r *Repo) Get() (RecordSet, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldUseCachedValues() {
		return r.cachedRecordSet, r.cachedRecordSetError
	}
	newRecordSet, err := r.createFromFileSystem()
	if err == nil {
		r.cachedRecordSet = newRecordSet
		r.cachedRecordSetError = err
	}

	return r.cachedRecordSet, r.cachedRecordSetError
}

func (r *Repo) shouldUseCachedValues() bool {
	if r.cachedRecordSetError != nil {
		return false
	}

	newStat, err := r.fileSystem.Stat(r.recordsFilePath)
	if err != nil {
		return true
	}

	unchanged := reflect.DeepEqual(r.stat, newStat)
	return unchanged
}

func (r *Repo) createFromFileSystem() (RecordSet, error) {
	info, err := r.fileSystem.Stat(r.recordsFilePath)
	if err != nil {
		return RecordSet{}, bosherr.Errorf("Error stating records file '%s':%s", r.recordsFilePath, err.Error())
	}

	buf, err := r.fileSystem.ReadFile(r.recordsFilePath)
	if err != nil {
		return RecordSet{}, err
	}

	r.stat = info

	var newRecordSet RecordSet
	if err := json.Unmarshal(buf, &newRecordSet); err != nil {
		return RecordSet{}, err
	}

	return newRecordSet, nil
}

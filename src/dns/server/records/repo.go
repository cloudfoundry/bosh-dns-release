package records

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"sync"
	"time"
)

const logTag string = "RecordsRepo"

type Repo struct {
	fileSystem           system.FileSystem
	recordsFilePath      string
	cachedRecordSetError error
	cachedRecordSet      *RecordSet
	lastReadTime         time.Time
	mutex                sync.Mutex
}

func NewRepo(recordsFilePath string, fileSys system.FileSystem, logger logger.Logger) *Repo {
	repo := Repo{
		recordsFilePath: recordsFilePath,
		fileSystem:      fileSys,
	}

	_, err := repo.Get()
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to open records file at: %s", recordsFilePath))
	}
	return &repo
}

func (r *Repo) Get() (RecordSet, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.shouldUseCachedValues() {
		return *r.cachedRecordSet, r.cachedRecordSetError
	}

	r.cachedRecordSet, r.cachedRecordSetError = r.createFromFileSystem()

	return *r.cachedRecordSet, r.cachedRecordSetError
}

func (r Repo) shouldUseCachedValues() bool {
	if r.cachedRecordSet == nil {
		return false
	}

	info, err := r.fileSystem.Stat(r.recordsFilePath)
	if err != nil {
		return true
	}

	if r.lastReadTime.Sub(info.ModTime()) < time.Second {
		return false
	}

	return true
}

func (r *Repo) createFromFileSystem() (*RecordSet, error) {
	found := r.fileSystem.FileExists(r.recordsFilePath)
	if !found {
		return &RecordSet{}, bosherr.Errorf("Records file '%s' not found", r.recordsFilePath)
	}
	r.lastReadTime = time.Now()

	buf, err := r.fileSystem.ReadFile(r.recordsFilePath)
	if err != nil {
		return &RecordSet{}, err
	}

	var records RecordSet
	if err := json.Unmarshal(buf, &records); err != nil {
		return &RecordSet{}, err
	}

	return &records, nil
}

package records

import (
	"encoding/json"
	"io/ioutil"
	"time"
	"os"
	"github.com/cloudfoundry/bosh-utils/logger"
	"fmt"
)

const logTag string = "RecordsRepo"

type Repo struct {
	recordsFilePath      string
	cachedRecordSetError error
	cachedRecordSet      *RecordSet
	lastReadTime         time.Time
}

func NewRepo(recordsFilePath string, logger logger.Logger) *Repo {
	repo := Repo{
		recordsFilePath: recordsFilePath,
	}

	_, err := repo.Get()
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to open records file at: %s", recordsFilePath))
	}
	return &repo
}

func (r *Repo) Get() (RecordSet, error) {
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

	info, err := os.Stat(r.recordsFilePath)
	if err != nil {
		return true
	}

	if r.lastReadTime.Sub(info.ModTime()) < time.Second {
		return false
	}

	return true
}

func (r *Repo) createFromFileSystem() (*RecordSet, error) {
	_, err := os.Open(r.recordsFilePath)
	if err != nil {
		return &RecordSet{}, err
	}
	r.lastReadTime = time.Now()

	buf, err := ioutil.ReadFile(r.recordsFilePath)
	if err != nil {
		return &RecordSet{}, err
	}

	var records RecordSet
	if err := json.Unmarshal(buf, &records); err != nil {
		return &RecordSet{}, err
	}

	return &records, nil
}

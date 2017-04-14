package records

import (
	"encoding/json"
	"io/ioutil"
)

type Repo struct {
	recordsFilePath string
}

func NewRepo(recordsFilePath string) Repo {
	return Repo{
		recordsFilePath: recordsFilePath,
	}
}

func (r Repo) Get() (RecordSet, error) {
	return r.createFromFileSystem()
}

func (r Repo) createFromFileSystem() (RecordSet, error) {
	buf, err := ioutil.ReadFile(r.recordsFilePath)
	if err != nil {
		return RecordSet{}, err
	}

	var records RecordSet
	if err := json.Unmarshal(buf, &records); err != nil {
		return RecordSet{}, err
	}

	return records, nil
}

package zone_pickers

import (
	"encoding/json"
	"io/ioutil"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"sync/atomic"
)

type ZoneFilePicker struct {
	Domains []string `json:"zones"`
	head    uint32
}

func NewZoneFilePickerFromFile(source string) (*ZoneFilePicker, error) {
	jsonBytes, err := ioutil.ReadFile(source)
	if err != nil {
		return nil, bosherr.WrapError(err, "Creating zone picker")
	}

	picker := ZoneFilePicker{}
	err = json.Unmarshal(jsonBytes, &picker)
	if err != nil {
		return nil, err
	}

	return &picker, nil
}

func (j *ZoneFilePicker) NextZone() string {
	head_threadsafe := atomic.LoadUint32(&j.head)
	idx := int(head_threadsafe) % len(j.Domains)
	atomic.AddUint32(&j.head, 1)

	return j.Domains[idx]
}

package zone_pickers

import (
	"encoding/json"
	"os"
	"sync/atomic"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type ZoneFilePicker struct {
	Domains []string `json:"zones"`
	head    uint32
}

func NewZoneFilePickerFromFile(source string) (*ZoneFilePicker, error) {
	jsonBytes, err := os.ReadFile(source)
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
	headThreadSafe := atomic.LoadUint32(&j.head)
	idx := int(headThreadSafe) % len(j.Domains)
	atomic.AddUint32(&j.head, 1)

	return j.Domains[idx]
}

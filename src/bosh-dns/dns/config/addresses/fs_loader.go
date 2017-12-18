package addresses

import (
	"encoding/json"
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type FSLoader struct {
	fs boshsys.FileSystem
}

func (l FSLoader) Load(filename string) (AddressConfigs, error) {
	fileContents, err := l.fs.ReadFile(filename)
	if err != nil {
		return nil, bosherr.WrapError(err, "missing address config file")
	}

	var addresses AddressConfigs
	err = json.Unmarshal(fileContents, &addresses)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "addresses config file malformed: %s", filename)
	}

	for _, c := range addresses {
		if c.Port == 0 {
			return nil, errors.New("port is required")
		}
	}

	return addresses, nil
}

func NewFSLoader(fs boshsys.FileSystem) FSLoader {
	return FSLoader{fs: fs}
}

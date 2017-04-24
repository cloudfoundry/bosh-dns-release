package aliases

import (
	"encoding/json"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type FSLoader struct {
	fs boshsys.FileSystem
}

func (l FSLoader) Load(filename string) (Config, error) {
	fileContents, err := l.fs.ReadFile(filename)
	if err != nil {
		return nil, bosherr.WrapError(err, "missing alias config file")
	}

	config := Config{}
	err = json.Unmarshal(fileContents, &config)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "alias config file malformed: %s", filename)
	}

	return config, nil
}

func NewFSLoader(fs boshsys.FileSystem) NamedConfigLoader {
	return FSLoader{fs: fs}
}

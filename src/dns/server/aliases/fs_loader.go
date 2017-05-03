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
		return Config{}, bosherr.WrapError(err, "missing alias config file")
	}

	cfg := Config{}
	err = json.Unmarshal(fileContents, &cfg)
	if err != nil {
		return cfg, bosherr.WrapErrorf(err, "alias config file malformed: %s", filename)
	}

	return cfg, nil
}

func NewFSLoader(fs boshsys.FileSystem) NamedConfigLoader {
	return FSLoader{fs: fs}
}

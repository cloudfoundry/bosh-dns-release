package handlers

import (
	"encoding/json"

	"bosh-dns/dns/config"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type FSLoader struct {
	fs boshsys.FileSystem
}

func (l FSLoader) Load(filename string) (HandlerConfigs, error) {
	fileContents, err := l.fs.ReadFile(filename)
	if err != nil {
		return nil, bosherr.WrapError(err, "missing handlers config file")
	}

	var handlers HandlerConfigs
	err = json.Unmarshal(fileContents, &handlers)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "handlers config file malformed: %s", filename)
	}

	for i := range handlers {
		handlers[i].Source.Recursors, err = config.AppendDefaultDNSPortIfMissing(handlers[i].Source.Recursors)
		if err != nil {
			return nil, err
		}
	}

	return handlers, nil
}

func NewFSLoader(fs boshsys.FileSystem) FSLoader {
	return FSLoader{fs: fs}
}

package aliases

import (
	"encoding/json"
	"maps"
	"slices"

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

type FSSuffixesLoader struct {
	fs boshsys.FileSystem
}

// Load reads a JSON object that maps alias hosts to suffixes. It returns an
// error when the file cannot be read or is not a JSON object. It returns a
// separate error for each entry that is not usable and keeps the rest.
func (l FSSuffixesLoader) Load(filename string) (Suffixes, []error, error) {
	fileContents, err := l.fs.ReadFile(filename)
	if err != nil {
		return NewSuffixes(), nil, bosherr.WrapError(err, "could not be read")
	}

	raw := map[string]interface{}{}
	err = json.Unmarshal(fileContents, &raw)
	if err != nil {
		return NewSuffixes(), nil, bosherr.WrapError(err, "is not a JSON object")
	}

	var errs []error
	load := map[string]string{}
	for _, rawAlias := range slices.Sorted(maps.Keys(raw)) {
		suffix, ok := raw[rawAlias].(string)
		if !ok {
			errs = append(errs, bosherr.Errorf("alias %q: the suffix is not a string", rawAlias))
			continue
		}
		load[rawAlias] = suffix
	}

	suffixes, entryErrs := NewSuffixesFromMap(load)

	return suffixes, append(errs, entryErrs...), nil
}

// NewFSSuffixesLoader returns a NamedSuffixesLoader that reads alias suffix
// files from fs.
func NewFSSuffixesLoader(fs boshsys.FileSystem) NamedSuffixesLoader {
	return FSSuffixesLoader{fs: fs}
}

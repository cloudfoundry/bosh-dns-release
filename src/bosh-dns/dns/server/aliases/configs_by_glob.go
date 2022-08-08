package aliases

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

//go:generate counterfeiter . ConfigGlobber

type ConfigGlobber interface {
	Glob(string) ([]string, error)
}

//go:generate counterfeiter . NamedConfigLoader

type NamedConfigLoader interface {
	Load(string) (Config, error)
}

func ConfigFromGlob(nameFinder ConfigGlobber, loader NamedConfigLoader, glob string) (Config, error) {
	files, err := nameFinder.Glob(glob)
	if err != nil {
		return Config{}, bosherr.WrapError(err, "glob pattern failed to compute")
	}

	aliasConfig := NewConfig()

	for _, aliasFile := range files {
		nextConfig, err := loader.Load(aliasFile)
		if err != nil {
			return Config{}, bosherr.WrapError(err, "could not load config")
		}
		aliasConfig = aliasConfig.Merge(nextConfig)
	}

	canonicalAliases, err := aliasConfig.ReducedForm()
	if err != nil {
		return Config{}, bosherr.WrapError(err, "could not produce valid alias config")
	}

	return canonicalAliases, nil
}

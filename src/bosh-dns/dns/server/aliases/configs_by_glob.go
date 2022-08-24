package aliases

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

//counterfeiter:generate . ConfigGlobber

type ConfigGlobber interface {
	Glob(string) ([]string, error)
}

//counterfeiter:generate . NamedConfigLoader

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

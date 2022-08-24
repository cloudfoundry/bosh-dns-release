package addresses

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
	Load(string) (AddressConfigs, error)
}

func ConfigFromGlob(globber ConfigGlobber, loader NamedConfigLoader, glob string) (AddressConfigs, error) {
	filePaths, err := globber.Glob(glob)

	if err != nil {
		return nil, bosherr.WrapError(err, "glob pattern failed to compute")
	}

	addresses := AddressConfigs{}

	for _, filePath := range filePaths {
		found, err := loader.Load(filePath)
		if err != nil {
			return nil, bosherr.WrapError(err, "could not load config")
		}

		addresses = append(addresses, found...)
	}

	return addresses, nil
}

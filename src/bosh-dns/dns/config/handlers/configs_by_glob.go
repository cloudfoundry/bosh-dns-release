package handlers

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

//counterfeiter:generate . ConfigGlobber

type ConfigGlobber interface {
	Glob(string) ([]string, error)
}

//counterfeiter:generate . NamedConfigLoader

type NamedConfigLoader interface {
	Load(string) (HandlerConfigs, error)
}

func ConfigFromGlob(globber ConfigGlobber, loader NamedConfigLoader, glob string) (HandlerConfigs, error) {
	filePaths, err := globber.Glob(glob)

	if err != nil {
		return nil, bosherr.WrapError(err, "glob pattern failed to compute")
	}

	handlers := HandlerConfigs{}

	for _, filePath := range filePaths {
		found, err := loader.Load(filePath)
		if err != nil {
			return nil, bosherr.WrapError(err, "could not load config")
		}

		handlers = append(handlers, found...)
	}

	return handlers, nil
}

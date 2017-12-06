package handlers

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

//go:generate counterfeiter . ConfigGlobber

type ConfigGlobber interface {
	Glob(string) ([]string, error)
}

//go:generate counterfeiter . NamedConfigLoader

type NamedConfigLoader interface {
	Load(string) (HandlersConfig, error)
}

func ConfigFromGlob(globber ConfigGlobber, loader NamedConfigLoader, glob string) (HandlersConfig, error) {
	filePaths, err := globber.Glob(glob)

	if err != nil {
		return HandlersConfig{}, bosherr.WrapError(err, "glob pattern failed to compute")
	}

	var handlers []HandlerConfig

	for _, filePath := range filePaths {
		found, err := loader.Load(filePath)
		if err != nil {
			return HandlersConfig{}, bosherr.WrapError(err, "could not load config")
		}

		handlers = append(handlers, found.Handlers...)
	}

	return HandlersConfig{Handlers: handlers}, nil
}

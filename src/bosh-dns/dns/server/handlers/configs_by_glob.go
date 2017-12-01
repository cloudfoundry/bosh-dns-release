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
	Load(string) (Config, error)
}

func ConfigFromGlob(globber ConfigGlobber, loader NamedConfigLoader, glob string) (Config, error) {
	filePaths, err := globber.Glob(glob)

	if err != nil {
		return Config{}, bosherr.WrapError(err, "glob pattern failed to compute")
	}

	var handlers []Handler

	for _, filePath := range filePaths {
		found, err := loader.Load(filePath)
		if err != nil {
			return Config{}, bosherr.WrapError(err, "could not load config")
		}

		handlers = append(handlers, found.Handlers...)
	}

	return Config{Handlers: handlers}, nil
}

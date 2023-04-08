package config

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"fmt"
	"math/rand"
)

//counterfeiter:generate . RecursorReader

type RecursorReader interface {
	Get() ([]string, error)
}

func ConfigureRecursors(reader RecursorReader, dnsConfig *Config) error {
	if dnsConfig == nil {
		return nil
	}

	if len(dnsConfig.Recursors) <= 0 {
		var err error
		dnsConfig.Recursors, err = reader.Get()
		if err != nil {
			return err
		}
	}

	recursors := []string{}

	for _, recursor := range dnsConfig.Recursors {
		shouldAdd := true

		for _, excludedRecursor := range dnsConfig.ExcludedRecursors {
			if recursor == excludedRecursor {
				shouldAdd = false

				break
			}
		}

		if shouldAdd {
			recursors = append(recursors, recursor)
		}
	}

	switch dnsConfig.RecursorSelection {
	case SmartRecursorSelection:
		rand.Shuffle(len(recursors), func(i, j int) {
			recursors[i], recursors[j] = recursors[j], recursors[i]
		})
		dnsConfig.Recursors = recursors
	case SerialRecursorSelection:
		dnsConfig.Recursors = recursors
	default:
		return fmt.Errorf("invalid value for recursor selection: '%s'", dnsConfig.RecursorSelection)
	}

	return nil
}

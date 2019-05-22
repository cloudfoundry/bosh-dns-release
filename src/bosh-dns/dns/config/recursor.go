package config

import "fmt"

//go:generate counterfeiter . RecursorReader

type RecursorReader interface {
	Get() ([]string, error)
}

//go:generate counterfeiter . StringShuffler
type StringShuffler interface {
	Shuffle(src []string) []string
}

func ConfigureRecursors(reader RecursorReader, shuffler StringShuffler, dnsConfig *Config) error {
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
	case "smart":
		dnsConfig.Recursors = shuffler.Shuffle(recursors)
	case "serial":
		dnsConfig.Recursors = recursors
	default:
		return fmt.Errorf("invalid value for recursor selection: '%s'", dnsConfig.RecursorSelection)
	}

	return nil
}

package config

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
		recursors, err := reader.Get()
		if err != nil {
			return err
		}

		recursors = shuffler.Shuffle(recursors)

		dnsConfig.Recursors = recursors
	}

	return nil
}

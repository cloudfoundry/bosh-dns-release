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

	recursors := dnsConfig.Recursors

	if len(dnsConfig.Recursors) <= 0 {
		var err error
		recursors, err = reader.Get()
		if err != nil {
			return err
		}
	}

	dnsConfig.Recursors = shuffler.Shuffle(recursors)

	return nil
}

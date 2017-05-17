package config

//go:generate counterfeiter . RecursorReader
type RecursorReader interface {
	Get() ([]string, error)
}

func ConfigureRecursors(reader RecursorReader, dnsConfig *Config) error {
	if dnsConfig == nil {
		return nil
	}

	if len(dnsConfig.Recursors) <= 0 {
		recursors, err := reader.Get()
		if err != nil {
			return err
		}

		dnsConfig.Recursors = recursors
	}

	return nil
}

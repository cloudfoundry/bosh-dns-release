package aliases

//go:generate counterfeiter . RecordSet

type RecordSet interface {
	Resolve(string) ([]string, error)
	Domains() []string
	Subscribe() <-chan bool
}

type AliasedRecordSet struct {
	recordSet RecordSet
	config    Config
}

func NewAliasedRecordSet(recordSet RecordSet, config Config) *AliasedRecordSet {
	return &AliasedRecordSet{
		recordSet: recordSet,
		config:    config,
	}
}

func (a *AliasedRecordSet) Resolve(domain string) ([]string, error) {
	resolutions := a.config.Resolutions(domain)
	if len(resolutions) > 0 {
		var err error
		ips := []string{}

		for _, resolution := range resolutions {
			var hostIPs []string
			hostIPs, err = a.recordSet.Resolve(resolution)
			ips = append(ips, hostIPs...)
		}

		if len(ips) == 0 && err != nil {
			return nil, err
		}
		return ips, nil
	}

	return a.recordSet.Resolve(domain)
}

func (a *AliasedRecordSet) Subscribe() <-chan bool {
	return a.recordSet.Subscribe()
}

func (a *AliasedRecordSet) Domains() []string {
	return append(a.recordSet.Domains(), a.config.AliasHosts()...)
}

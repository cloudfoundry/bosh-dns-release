package config

import (
	"fmt"

	"bosh-dns/dns/manager"
)

const loopbackAddress = "127.0.0.1"

func NewRecursorReader(dnsManager manager.DNSManager, dnsNameServer string) recursorReader {
	return recursorReader{
		manager:       dnsManager,
		dnsNameServer: dnsNameServer,
	}
}

type recursorReader struct {
	manager       manager.DNSManager
	dnsNameServer string
}

func (r recursorReader) Get() ([]string, error) {
	recursors := []string{}

	nameservers, err := r.manager.Read()
	if err != nil {
		return nil, err
	}

	for _, server := range nameservers {
		if server != r.dnsNameServer && server != loopbackAddress {
			recursors = append(recursors, fmt.Sprintf("%s:53", server))
		}
	}

	return recursors, nil
}

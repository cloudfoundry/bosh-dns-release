package config

import (
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
	nameservers, err := r.manager.Read()
	if err != nil {
		return nil, err
	}

	validRecursors := []string{}
	for _, server := range nameservers {
		if r.isValid(server) {
			validRecursors = append(validRecursors, server)
		}
	}

	return AppendDefaultDNSPortIfMissing(validRecursors)
}

func (r recursorReader) isNameServer(s string) bool {
	return r.dnsNameServer == s
}

func (r recursorReader) isValid(server string) bool {
	return !r.isNameServer(server) &&
		server != loopbackAddress &&
		server != ""
}

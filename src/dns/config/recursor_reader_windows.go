// +build windows

package config

import (
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func NewResolvConfRecursorReader(fs boshsys.FileSystem, dnsServerNameServer string) ResolvConfRecursorReader {
	return ResolvConfRecursorReader{}
}

type ResolvConfRecursorReader struct {
}

func (r ResolvConfRecursorReader) Get() ([]string, error) {
	return []string{}, nil
}

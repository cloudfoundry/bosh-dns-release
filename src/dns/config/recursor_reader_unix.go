// +build linux darwin

package config

import (
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	"strings"
	"regexp"
)

func NewResolvConfRecursorReader(fs boshsys.FileSystem, dnsServerNameServer string) ResolvConfRecursorReader {
	return ResolvConfRecursorReader{
		fs:                  fs,
		dnsServerNameServer: dnsServerNameServer,
	}
}

type ResolvConfRecursorReader struct {
	fs                  boshsys.FileSystem
	dnsServerNameServer string
}

func (r ResolvConfRecursorReader) Get() ([]string, error) {
	nameserverRegexp, err := regexp.Compile("^\\s*nameserver\\s+(\\S+)$")
	if err != nil {
		return nil, err
	}

	recursors := []string{}
	contents, err := r.fs.ReadFileString("/etc/resolv.conf")

	if err != nil {
		return nil, bosherr.WrapError(err, "attempting to read recursors")
	}

	resolvConfLines := strings.Split(contents, "\n")
	for _, line := range resolvConfLines {
		submatch := nameserverRegexp.FindAllStringSubmatch(line, 1)

		if len(submatch) > 0 && !r.isDnsAddress(submatch[0][1]) {
			recursors = append(recursors, submatch[0][1]+":53")
		}
	}

	return recursors, nil
}

const loopbackAddress = "127.0.0.1"

func (r ResolvConfRecursorReader) isDnsAddress(nameserver string) bool {
	return nameserver == r.dnsServerNameServer || nameserver == loopbackAddress
}

package manager

import (
	"regexp"
	"strings"

	"code.cloudfoundry.org/clock"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type resolvConfManager struct {
	address   string
	fs        boshsys.FileSystem
	cmdRunner boshsys.CmdRunner
	clock     clock.Clock
}

var nameserverRegexp = regexp.MustCompile("^\\s*nameserver\\s+(\\S+)$")

func NewResolvConfManager(
	address string,
	clock clock.Clock,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner) *resolvConfManager {
	return &resolvConfManager{
		address:   address,
		fs:        fs,
		cmdRunner: cmdRunner,
		clock:     clock,
	}
}

func (r *resolvConfManager) Read() ([]string, error) {
	nameservers := []string{}
	contents, err := r.fs.ReadFileWithOpts("/etc/resolv.conf", boshsys.ReadOpts{Quiet: true})

	if err != nil {
		return nil, bosherr.WrapError(err, "attempting to read dns nameservers")
	}

	resolvConfLines := strings.Split(string(contents), "\n")
	for _, line := range resolvConfLines {
		submatch := nameserverRegexp.FindAllStringSubmatch(line, 1)

		if len(submatch) > 0 {
			nameservers = append(nameservers, submatch[0][1])
		}
	}

	return nameservers, nil
}

func (r *resolvConfManager) SetPrimary() error {
	_, _, _, err := r.cmdRunner.RunCommand("resolvconf-manager", "-head", r.address)
	if err != nil {
		return bosherr.WrapError(err, "Executing resolvconf-manager")
	}

	return err
}

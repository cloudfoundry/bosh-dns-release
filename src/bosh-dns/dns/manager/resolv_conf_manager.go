package manager

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const MaxResolvConfRetries = 8

var warningLine = "# This file was automatically updated by bosh-dns"

var nameserverLineRegex = regexp.MustCompile("^nameserver (.+)")

type resolvConfManager struct {
	fs        boshsys.FileSystem
	cmdRunner boshsys.CmdRunner
	clock     clock.Clock
}

func NewResolvConfManager(clock clock.Clock, fs boshsys.FileSystem, cmdRunner boshsys.CmdRunner) *resolvConfManager {
	return &resolvConfManager{
		fs:        fs,
		cmdRunner: cmdRunner,
		clock:     clock,
	}
}

func (r *resolvConfManager) Read() ([]string, error) {
	nameserverRegexp, err := regexp.Compile("^\\s*nameserver\\s+(\\S+)$")
	if err != nil {
		return nil, err
	}

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

func (r *resolvConfManager) SetPrimary(address string) error {
	writeString := fmt.Sprintf(`%s
nameserver %s
`, warningLine, address)

	if correct, _ := r.isCorrect(address); correct {
		return nil
	}

	if r.fs.FileExists("/etc/resolvconf/resolv.conf.d/head") {
		append, err := r.fs.ReadFileString("/etc/resolvconf/resolv.conf.d/head")
		if err != nil {
			return bosherr.WrapError(err, "Reading existing head")
		}

		if !r.isStringCorrect(address, append) {
			writeString = fmt.Sprintf("%s\n%s", writeString, append)
		}
	}

	err := r.fs.WriteFileString("/etc/resolvconf/resolv.conf.d/head", writeString)
	if err != nil {
		return bosherr.WrapError(err, "Writing head")
	}

	_, _, _, err = r.cmdRunner.RunCommand("resolvconf", "-u")
	if err != nil {
		return bosherr.WrapError(err, "Executing resolvconf")
	}

	for i := 0; i < MaxResolvConfRetries; i++ {
		if correct, _ := r.isCorrect(address); correct {
			return nil
		}

		// seems like `resolvconf -u` may not immediately update /etc/resolv.conf, so
		// block here briefly to try and ensure it was successful before we error
		r.clock.Sleep(2 * time.Second)
	}

	return errors.New("Failed to confirm nameserver in /etc/resolv.conf")
}

func (r *resolvConfManager) isCorrect(address string) (bool, error) {
	servers, err := r.Read()
	if err != nil {
		return false, err
	}

	for _, server := range servers {
		if server == address {
			return true, nil
		}

		return false, nil
	}

	return false, nil
}

func (r resolvConfManager) isStringCorrect(address, contents string) bool {
	lines := strings.Split(contents, "\n")

	for _, l := range lines {
		if !nameserverLineRegex.MatchString(l) {
			continue
		}

		if l == fmt.Sprintf("nameserver %s", address) {
			return true
		}

		return false
	}

	return false
}

package handler

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

var warningLine = "# This file was automatically updated by bosh-dns"

var nameserverLineRegex = regexp.MustCompile("^nameserver (.+)")

type ResolvConfHandler struct {
	address   string
	clock     clock.Clock
	fs        boshsys.FileSystem
	cmdRunner boshsys.CmdRunner
}

func NewResolvConfHandler(address string, clock clock.Clock, fileSys boshsys.FileSystem, cmdRunner boshsys.CmdRunner) ResolvConfHandler {
	return ResolvConfHandler{
		address:   address,
		clock:     clock,
		fs:        fileSys,
		cmdRunner: cmdRunner,
	}
}

func (c ResolvConfHandler) Apply() error {
	writeString := fmt.Sprintf(`%s
nameserver %s
`, warningLine, c.address)

	if c.fs.FileExists("/etc/resolvconf/resolv.conf.d/head") {
		append, err := c.fs.ReadFileString("/etc/resolvconf/resolv.conf.d/head")
		if err != nil {
			return bosherr.WrapError(err, "Reading existing head")
		}

		if !c.isStringCorrect(append) {
			writeString = fmt.Sprintf("%s\n%s", writeString, append)
		}
	}

	err := c.fs.WriteFileString("/etc/resolvconf/resolv.conf.d/head", writeString)
	if err != nil {
		return bosherr.WrapError(err, "Writing head")
	}

	_, _, _, err = c.cmdRunner.RunCommand("resolvconf", "-u")
	if err != nil {
		return bosherr.WrapError(err, "Executing resolvconf")
	}

	for i := 0; i < 8; i++ {
		if correct, _ := c.IsCorrect(); correct {
			return nil
		}

		// seems like `resolvconf -u` may not immediately update /etc/resolv.conf, so
		// block here briefly to try and ensure it was successful before we error
		c.clock.Sleep(2 * time.Second)
	}

	return errors.New("Failed to confirm nameserver in /etc/resolv.conf")
}

func (c ResolvConfHandler) IsCorrect() (bool, error) {
	contents, err := c.fs.ReadFileString("/etc/resolv.conf")
	if err != nil {
		return false, bosherr.WrapError(err, "Reading resolv.conf")
	}

	return c.isStringCorrect(contents), nil
}

func (c ResolvConfHandler) isStringCorrect(contents string) bool {
	lines := strings.Split(contents, "\n")

	for _, l := range lines {
		if !nameserverLineRegex.MatchString(l) {
			continue
		}

		if l == fmt.Sprintf("nameserver %s", c.address) {
			return true
		}

		return false
	}

	return false
}

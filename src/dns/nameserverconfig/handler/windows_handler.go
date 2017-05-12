package handler

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"strings"
)

type WindowsHandler struct {
	address   string
	cmdRunner boshsys.CmdRunner
}

func NewWindowsHandler(address string, cmdRunner boshsys.CmdRunner) WindowsHandler {
	return WindowsHandler{
		address:   address,
		cmdRunner: cmdRunner,
	}
}

func (c WindowsHandler) Apply() error {
	_, _, _, err := c.cmdRunner.RunCommand("powershell.exe", "/var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1", c.address)
	if err != nil {
		return bosherr.WrapError(err, "Executing prepend-dns-server.ps1")
	}

	return nil
}

func (c WindowsHandler) IsCorrect() (bool, error) {
	stdout, _, _, err := c.cmdRunner.RunCommand("powershell.exe", "/var/vcap/packages/dns-windows/bin/list-server-addresses.ps1")
	if err != nil {
		return false, bosherr.WrapError(err, "Executing list-server-addresses.ps1")
	}

	servers := strings.SplitN(stdout, "\r\n", 2)

	return servers[0] == c.address, nil
}

// +build windows

package main

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/handler"
)

func HandlerFactory(address string, _ boshlog.Logger, runner boshsys.CmdRunner) handler.Handler {
	return handler.NewWindowsHandler(address, runner)
}

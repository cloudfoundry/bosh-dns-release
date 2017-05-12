// +build !windows

package main

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/handler"
)

func HandlerFactory(bindAddress string, logger boshlog.Logger, cmdRunner boshsys.CmdRunner) handler.Handler {
	return handler.NewResolvConfHandler(bindAddress, boshsys.NewOsFileSystem(logger), cmdRunner)
}

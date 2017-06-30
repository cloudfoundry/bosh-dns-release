package main

import (
	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"dns/manager"
)

func newDNSManager(logger logger.Logger, _ clock.Clock, _ boshsys.FileSystem) manager.DNSManager {
	return manager.NewWindowsManager(boshsys.NewExecCmdRunner(logger))
}

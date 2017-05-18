package main

import (
	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/dns-release/src/dns/manager"
)

func newDNSManager(logger logger.Logger, clock clock.Clock, fs boshsys.FileSystem) manager.DNSManager {
	return manager.NewResolvConfManager(clock, fs, boshsys.NewExecCmdRunner(logger))
}

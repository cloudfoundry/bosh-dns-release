//+build !windows

package main

import (
	"bosh-dns/dns/manager"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func newDNSManager(logger logger.Logger, clock clock.Clock, fs boshsys.FileSystem) manager.DNSManager {
	return manager.NewResolvConfManager(clock, fs, boshsys.NewExecCmdRunner(logger))
}

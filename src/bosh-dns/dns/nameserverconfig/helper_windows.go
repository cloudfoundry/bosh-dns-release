package main

import (
	"bosh-dns/dns/manager"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func newDNSManager(logger logger.Logger, _ clock.Clock, fs boshsys.FileSystem) manager.DNSManager {
	return manager.NewWindowsManager(boshsys.NewExecCmdRunner(logger), fs, manager.WindowsAdapterFetcher{})
}

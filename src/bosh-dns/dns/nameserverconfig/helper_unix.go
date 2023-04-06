//go:build !windows
// +build !windows

package main

import (
	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-dns/dns/manager"
)

func newDNSManager(address string, logger logger.Logger, clock clock.Clock, fs boshsys.FileSystem) manager.DNSManager {
	return manager.NewResolvConfManager(address, clock, fs, boshsys.NewExecCmdRunner(logger))
}

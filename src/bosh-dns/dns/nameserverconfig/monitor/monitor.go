package monitor

import (
	"bosh-dns/dns/manager"

	"code.cloudfoundry.org/clock"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type Monitor struct {
	logger     boshlog.Logger
	address    string
	dnsManager manager.DNSManager
	signal     clock.Ticker
}

func NewMonitor(logger boshlog.Logger, address string, dnsManager manager.DNSManager, signal clock.Ticker) Monitor {
	return Monitor{
		logger:     logger,
		address:    address,
		dnsManager: dnsManager,
		signal:     signal,
	}
}

func (c Monitor) RunOnce() error {
	err := c.dnsManager.SetPrimary(c.address)
	if err != nil {
		return bosherr.WrapError(err, "Updating nameserver configs")
	}

	return nil
}

func (c Monitor) Run(shutdown chan struct{}) {
	run := c.signal.C()
	for {
		select {
		case <-shutdown:
			return
		case <-run:
			err := c.RunOnce()
			if err != nil {
				c.logger.Error("NameserverConfigMonitor", "running: %s", err)
			}
		}
	}
}

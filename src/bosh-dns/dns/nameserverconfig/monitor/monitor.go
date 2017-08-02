package monitor

import (
	"time"

	"bosh-dns/dns/manager"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type Monitor struct {
	logger     boshlog.Logger
	address    string
	dnsManager manager.DNSManager
	interval   time.Duration
}

func NewMonitor(logger boshlog.Logger, address string, dnsManager manager.DNSManager, interval time.Duration) Monitor {
	return Monitor{
		logger:     logger,
		address:    address,
		dnsManager: dnsManager,
		interval:   interval,
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
	for {
		select {
		case <-time.After(c.interval):
			err := c.RunOnce()
			if err != nil {
				c.logger.Error("NameserverConfigMonitor", "running: %s", err)
			}
		case <-shutdown:
			return
		}
	}
}

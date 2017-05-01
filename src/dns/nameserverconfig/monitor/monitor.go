package monitor

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/handler"
	"time"
)

type Monitor struct {
	checker  handler.Handler
	logger   boshlog.Logger
	interval time.Duration
}

func NewMonitor(checker handler.Handler, logger boshlog.Logger, interval time.Duration) Monitor {
	return Monitor{
		checker:  checker,
		logger:   logger,
		interval: interval,
	}
}

func (c Monitor) RunOnce() error {
	good, err := c.checker.IsCorrect()
	if err != nil {
		return bosherr.WrapError(err, "Checking nameserver configs")
	}

	if !good {
		c.logger.Info("NameserverConfigMonitor", "Updating nameserver configs because of change")
		err = c.checker.Apply()
		if err != nil {
			return bosherr.WrapError(err, "Updating nameserver configs")
		}
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

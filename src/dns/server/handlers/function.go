package handlers

import (
	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

func AddHandler(mux ServerMux, clock clock.Clock, pattern string, handler dns.Handler, logger boshlog.Logger) {
	mux.Handle(pattern, NewRequestLoggerHandler(handler, clock, logger))
}

package handlers

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/miekg/dns"
)

func AddHandler(mux ServerMux, pattern string, handler dns.Handler, logger boshlog.Logger) {
	mux.Handle(pattern, NewRequestLoggerHandler(handler, clock.Real, logger))
}

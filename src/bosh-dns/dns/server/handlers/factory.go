package handlers

import (
	"bosh-dns/dns/shuffle"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type Factory struct {
	exchangerFactory ExchangerFactory
	clock            clock.Clock
	shuffler         shuffle.StringShuffle
	logger           boshlog.Logger
}

func NewFactory(exchangerFactory ExchangerFactory, clock clock.Clock, shuffler shuffle.StringShuffle, logger boshlog.Logger) *Factory {
	return &Factory{
		exchangerFactory: exchangerFactory,
		clock:            clock,
		shuffler:         shuffler,
		logger:           logger,
	}
}

func (f *Factory) CreateHTTPJSONHandler(url string, cache bool) dns.Handler {
	var handler dns.Handler

	httpClient := httpclient.NewHTTPClient(httpclient.DefaultClient, f.logger)
	handler = NewHTTPJSONHandler(url, httpClient, f.logger)

	if cache {
		handler = NewCachingDNSHandler(handler)
	}
	return handler
}

func (f *Factory) CreateForwardHandler(recursors []string, cache bool) dns.Handler {
	var handler dns.Handler
	pool := NewFailoverRecursorPool(f.shuffler.Shuffle(recursors), f.logger)
	handler = NewForwardHandler(pool, f.exchangerFactory, f.clock, f.logger)

	if cache {
		handler = NewCachingDNSHandler(handler)
	}
	return handler
}

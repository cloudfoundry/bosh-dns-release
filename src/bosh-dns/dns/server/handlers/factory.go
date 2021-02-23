package handlers

import (
	"bosh-dns/dns/config"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/shuffle"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type Factory struct {
	exchangerFactory   ExchangerFactory
	clock              clock.Clock
	shuffler           shuffle.StringShuffle
	recursorRetryCount int
	logger             boshlog.Logger
	truncater          dnsresolver.ResponseTruncater
}

func NewFactory(exchangerFactory ExchangerFactory, clock clock.Clock, shuffler shuffle.StringShuffle, recursorRetryCount int, logger boshlog.Logger, truncater dnsresolver.ResponseTruncater) *Factory {
	return &Factory{
		exchangerFactory:   exchangerFactory,
		clock:              clock,
		shuffler:           shuffler,
		recursorRetryCount: recursorRetryCount,
		logger:             logger,
		truncater:          truncater,
	}
}

func (f *Factory) CreateHTTPJSONHandler(url string, cache bool) dns.Handler {
	var handler dns.Handler

	httpClient := httpclient.NewHTTPClient(httpclient.DefaultClient, f.logger)
	handler = NewHTTPJSONHandler(url, httpClient, f.logger, f.truncater)

	if cache {
		handler = NewCachingDNSHandler(handler, f.truncater, f.clock, f.logger)
	}
	return handler
}

func (f *Factory) CreateForwardHandler(recursors []string, cache bool) dns.Handler {
	var handler dns.Handler

	// Forward handlers are not treated the same as recursors in
	// /etc/resolv.conf.
	//
	// The default behavior defined by DNS spec is to use
	// "smart" recursor selection.
	pool := NewFailoverRecursorPool(f.shuffler.Shuffle(recursors), config.SmartRecursorSelection, f.recursorRetryCount, f.logger)
	handler = NewForwardHandler(pool, f.exchangerFactory, f.clock, f.logger, f.truncater)

	if cache {
		handler = NewCachingDNSHandler(handler, f.truncater, f.clock, f.logger)
	}
	return handler
}

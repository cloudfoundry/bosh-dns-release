package handlers

import (
	"bosh-dns/dns/shuffle"
	"fmt"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/miekg/dns"
)

type Config struct {
	Handlers []DelegatingHandlerDescription `json:"handlers"`
}

type DelegatingHandlerDescription struct {
	Domain string      `json:"domain"`
	Source Source      `json:"source"`
	Cache  ConfigCache `json:"cache"`
}

type Source struct {
	Type      string   `json:"type"`
	URL       string   `json:"url,omitempty"`
	Recursors []string `json:"recursors,omitempty"`
}

type ConfigCache struct {
	Enabled bool `json:"enabled"`
}

func (c Config) RealHandlers(logger boshlog.Logger, stringShuffler shuffle.StringShuffle, clock clock.Clock, exchangerFactory ExchangerFactory) (map[string]dns.Handler, error) {
	var realHandlers = make(map[string]dns.Handler)
	for _, handlerConfig := range c.Handlers {
		var handler dns.Handler

		if handlerConfig.Source.Type == "http" {
			handler = NewHTTPJSONHandler(handlerConfig.Source.URL, logger)
		} else if handlerConfig.Source.Type == "dns" {
			if len(handlerConfig.Source.Recursors) == 0 {
				return make(map[string]dns.Handler), fmt.Errorf(`Configuring handler for "%s": No recursors present`, handlerConfig.Domain)
			}
			recursorPool := NewFailoverRecursorPool(stringShuffler.Shuffle(handlerConfig.Source.Recursors), logger)
			handler = NewForwardHandler(recursorPool, exchangerFactory, clock, logger)
		} else {
			return make(map[string]dns.Handler), fmt.Errorf(`Configuring handler for "%s": Unexpected handler source type: %s`, handlerConfig.Domain, handlerConfig.Source.Type)
		}

		if handlerConfig.Cache.Enabled {
			handler = NewCachingDNSHandler(handler)
		}
		realHandlers[handlerConfig.Domain] = handler
	}
	return realHandlers, nil
}

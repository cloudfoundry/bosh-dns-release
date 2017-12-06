package handlers

import (
	"fmt"

	"github.com/miekg/dns"
)

//go:generate counterfeiter . dnsHandler

type dnsHandler interface {
	dns.Handler
}

//go:generate counterfeiter . HandlerFactory
type HandlerFactory interface {
	CreateHTTPJSONHandler(string, bool) dns.Handler
	CreateForwardHandler([]string, bool) dns.Handler
}

type HandlersConfig struct {
	Handlers []HandlerConfig `json:"handlers"`
}

type HandlerConfig struct {
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

func (c HandlersConfig) GenerateHandlers(factory HandlerFactory) (map[string]dns.Handler, error) {
	var realHandlers = make(map[string]dns.Handler)
	for _, handlerConfig := range c.Handlers {
		var handler dns.Handler

		if handlerConfig.Source.Type == "http" {
			url := handlerConfig.Source.URL
			if url == "" {
				return nil, fmt.Errorf(`Configuring handler for "%s": HTTP handler must receive a URL`, handlerConfig.Domain)
			}

			handler = factory.CreateHTTPJSONHandler(url, handlerConfig.Cache.Enabled)
		} else if handlerConfig.Source.Type == "dns" {
			if len(handlerConfig.Source.Recursors) == 0 {
				return nil, fmt.Errorf(`Configuring handler for "%s": No recursors present`, handlerConfig.Domain)
			}

			handler = factory.CreateForwardHandler(handlerConfig.Source.Recursors, handlerConfig.Cache.Enabled)
		} else {
			return nil, fmt.Errorf(`Configuring handler for "%s": Unexpected handler source type: %s`, handlerConfig.Domain, handlerConfig.Source.Type)
		}

		realHandlers[handlerConfig.Domain] = handler
	}
	return realHandlers, nil
}

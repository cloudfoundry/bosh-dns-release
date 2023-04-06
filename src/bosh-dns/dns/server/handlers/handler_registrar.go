package handlers

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"

	"bosh-dns/dns/server/criteria"
)

const RegisterInterval = time.Millisecond * 250

//counterfeiter:generate . ServerMux

type ServerMux interface {
	Handle(pattern string, handler dns.Handler)
	HandleRemove(pattern string)
}

//counterfeiter:generate . DomainProvider

type DomainProvider interface {
	Domains() []string
}

type HandlerRegistrar struct {
	logger         logger.Logger
	clock          clock.Clock
	domainProvider DomainProvider
	mux            ServerMux
	handler        dns.Handler
	domains        map[string]struct{}
}

func NewHandlerRegistrar(logger logger.Logger, clock clock.Clock, domainProvider DomainProvider, mux ServerMux, handler dns.Handler) HandlerRegistrar {
	return HandlerRegistrar{
		logger:         logger,
		clock:          clock,
		domainProvider: domainProvider,
		mux:            mux,
		handler:        handler,
		domains:        map[string]struct{}{},
	}
}

func (h *HandlerRegistrar) RegisterAgentTLD() {
	h.mux.Handle(criteria.BoshAgentTLD, NewRequestLoggerHandler(h.handler, h.clock, h.logger))
}

// not threadsafe - only call externally before Run is called
func (h *HandlerRegistrar) UpdateDomainRegistrations() {
	currentDomains := make(map[string]struct{}, len(h.domains))
	for domain := range h.domains {
		currentDomains[domain] = struct{}{}
	}

	for _, domain := range h.domainProvider.Domains() {
		delete(currentDomains, domain)

		if _, ok := h.domains[domain]; !ok {
			h.logger.Info("HandlerRegistrar", "Register %s as internal domain", domain)
			h.domains[domain] = struct{}{}
			h.mux.Handle(domain, NewRequestLoggerHandler(h.handler, h.clock, h.logger))
		}
	}

	for domain := range currentDomains {
		h.logger.Info("HandlerRegistrar", "Unregister %s as internal domain", domain)
		delete(h.domains, domain)
		h.mux.HandleRemove(domain)
	}
}

func (h *HandlerRegistrar) Run(signal chan struct{}) error {
	ticker := h.clock.NewTicker(RegisterInterval)

	for {
		select {
		case <-signal:
			return nil
		case <-ticker.C():
			h.UpdateDomainRegistrations()
		}
	}
}

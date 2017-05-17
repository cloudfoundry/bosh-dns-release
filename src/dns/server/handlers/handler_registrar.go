package handlers

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"
)

const RegisterInterval = time.Millisecond * 250

//go:generate counterfeiter . ServerMux
type ServerMux interface {
	Handle(pattern string, handler dns.Handler)
	HandleRemove(pattern string)
}

type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

type HandlerRegistrar struct {
	logger      logger.Logger
	clock       clock.Clock
	recordsRepo RecordSetRepo
	mux         ServerMux
	handler     dns.Handler
	domains     map[string]struct{}
}

func NewHandlerRegistrar(logger logger.Logger, clock clock.Clock, recordsRepo RecordSetRepo, mux ServerMux, handler dns.Handler) HandlerRegistrar {
	return HandlerRegistrar{
		logger:      logger,
		clock:       clock,
		recordsRepo: recordsRepo,
		mux:         mux,
		handler:     handler,
		domains:     map[string]struct{}{},
	}
}

func (h *HandlerRegistrar) Run(signal chan struct{}) error {
	ticker := h.clock.NewTicker(RegisterInterval)
	for {
		select {
		case <-signal:
			return nil
		case <-ticker.C():
			recordSet, err := h.recordsRepo.Get()
			if err != nil {
				h.logger.Error("handler-registrar", "cannot get record set", err)
				continue
			}

			currentDomains := make(map[string]struct{}, len(h.domains))
			for domain := range h.domains {
				currentDomains[domain] = struct{}{}
			}

			for _, domain := range recordSet.Domains {
				delete(currentDomains, domain)

				if _, ok := h.domains[domain]; !ok {
					h.domains[domain] = struct{}{}
					AddHandler(h.mux, domain, h.handler, h.logger)
				}
			}

			for domain, _ := range currentDomains {
				delete(h.domains, domain)
				h.mux.HandleRemove(domain)
			}
		}
	}
}

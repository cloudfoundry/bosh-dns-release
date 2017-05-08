package handlers

import (
	"errors"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	"fmt"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
	"github.com/miekg/dns"
	"net"
	"strings"
)

type AliasResolvingHandler struct {
	child          dns.Handler
	config         aliases.Config
	domainResolver DomainResolver
	clock          clock.Clock
	logger         logger.Logger
	logTag         string
}

//go:generate counterfeiter . DomainResolver
type DomainResolver interface {
	ResolveAnswer(questionDomains []string, protocol dnsresolver.Protocol, requestMsg *dns.Msg) *dns.Msg
}

func NewAliasResolvingHandler(child dns.Handler, config aliases.Config, domainResolver DomainResolver, clock clock.Clock, logger logger.Logger) (AliasResolvingHandler, error) {
	if !config.IsReduced() {
		return AliasResolvingHandler{}, errors.New("must configure with non-recursing alias config")
	}

	return AliasResolvingHandler{
		child:          child,
		config:         config,
		domainResolver: domainResolver,
		clock:          clock,
		logger:         logger,
		logTag:         "AliasResolvingHandler",
	}, nil
}

func (h AliasResolvingHandler) ServeDNS(responseWriter dns.ResponseWriter, requestMsg *dns.Msg) {

	//don't find aliases when empty questions

	if aliasTargets := h.config.Resolutions(requestMsg.Question[0].Name); len(aliasTargets) > 0 {
		if len(aliasTargets) == 1 && aliasTargets[0] == "healthcheck.bosh-dns." {
			healthCheckHandler := NewHealthCheckHandler(h.logger)
			NewRequestLoggerHandler(healthCheckHandler, clock.Real, h.logger).ServeDNS(responseWriter, requestMsg)
			return
		}

		//add tests for protocol
		protocol := dnsresolver.UDP
		if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); ok {
			protocol = dnsresolver.TCP
		}

		before := h.clock.Now()

		responseMsg := h.domainResolver.ResolveAnswer(aliasTargets, protocol, requestMsg)
		rcode := responseMsg.Rcode

		if err := responseWriter.WriteMsg(responseMsg); err != nil {
			h.logger.Error(h.logTag, "error writing response %s", err.Error())
		}

		duration := h.clock.Now().Sub(before).Nanoseconds()

		types := make([]string, len(requestMsg.Question))
		domains := make([]string, len(requestMsg.Question))
		for i, q := range requestMsg.Question {
			types[i] = fmt.Sprintf("%d", q.Qtype)
			domains[i] = q.Name
		}
		h.logger.Info(h.logTag, fmt.Sprintf("%T Request [%s] [%s] %d %dns",
			h.domainResolver,
			strings.Join(types, ","),
			strings.Join(domains, ","),
			rcode,
			duration,
		))

		return
	}

	h.child.ServeDNS(responseWriter, requestMsg)
}

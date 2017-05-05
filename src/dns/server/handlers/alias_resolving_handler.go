package handlers

import (
	"errors"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	"github.com/miekg/dns"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
	"net"
)

type AliasResolvingHandler struct {
	child          dns.Handler
	config         aliases.Config
	domainResolver DomainResolver
	logger         logger.Logger
	logTag         string
}

//go:generate counterfeiter . DomainResolver
type DomainResolver interface {
	ResolveAnswer(answerDomain string, questionDomains []string, protocol dnsresolver.Protocol,requestMsg *dns.Msg) *dns.Msg
}

func NewAliasResolvingHandler(child dns.Handler, config aliases.Config, domainResolver DomainResolver, logger logger.Logger) (AliasResolvingHandler, error) {
	if !config.IsReduced() {
		return AliasResolvingHandler{}, errors.New("must configure with non-recursing alias config")
	}

	return AliasResolvingHandler{
		child:          child,
		config:         config,
		domainResolver: domainResolver,
		logger:         logger,
		logTag:         "AliasResolvingHandler",
	}, nil
}

func (h AliasResolvingHandler) ServeDNS(resp dns.ResponseWriter, requestMsg *dns.Msg) {
	if aliasTargets := h.config.Resolutions(requestMsg.Question[0].Name); len(aliasTargets) > 0 {

		protocol := dnsresolver.UDP

		if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
			protocol = dnsresolver.TCP
		}

		//add tests for protocol

		responseMsg := h.domainResolver.ResolveAnswer(requestMsg.Question[0].Name, aliasTargets, protocol, requestMsg)

		if err := resp.WriteMsg(responseMsg); err != nil {
			h.logger.Error(h.logTag, "error writing response %s", err.Error())
		}
		return
	}

	h.child.ServeDNS(resp, requestMsg)
}

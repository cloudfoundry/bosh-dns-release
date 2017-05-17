package handlers

import (
	"errors"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	"fmt"
	"strings"

	"github.com/miekg/dns"
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
	Resolve(questionDomains []string, responseWriter dns.ResponseWriter, requestMsg *dns.Msg) *dns.Msg
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
	questionName := ""
	if len(requestMsg.Question) > 0 {
		questionName = requestMsg.Question[0].Name
	}

	if aliasTargets := h.config.Resolutions(questionName); len(aliasTargets) > 0 {
		before := h.clock.Now()

		responseMsg := h.domainResolver.Resolve(aliasTargets, responseWriter, requestMsg)
		rcode := responseMsg.Rcode

		if err := responseWriter.WriteMsg(responseMsg); err != nil {
			h.logger.Error(h.logTag, "error writing response %s", err.Error())
		}

		duration := h.clock.Now().Sub(before).Nanoseconds()

		h.log(requestMsg, rcode, duration)

		return
	}

	h.child.ServeDNS(responseWriter, requestMsg)
}

func (h AliasResolvingHandler) log(requestMsg *dns.Msg, rcode int, duration int64) {
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
}

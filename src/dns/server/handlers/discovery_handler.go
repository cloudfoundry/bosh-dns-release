package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
	"github.com/miekg/dns"
)

type DiscoveryHandler struct {
	logger      logger.Logger
	logTag      string
	localDomain dnsresolver.LocalDomain
}

func NewDiscoveryHandler(logger logger.Logger, localDomain dnsresolver.LocalDomain) DiscoveryHandler {
	return DiscoveryHandler{
		logger:      logger,
		logTag:      "DiscoveryHandler",
		localDomain: localDomain,
	}
}

func (d DiscoveryHandler) ServeDNS(responseWriter dns.ResponseWriter, requestMsg *dns.Msg) {
	responseMsg := &dns.Msg{}
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = false

	if len(requestMsg.Question) == 0 {
		responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)
	} else {
		switch requestMsg.Question[0].Qtype {
		case dns.TypeMX, dns.TypeAAAA:
			responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)
		case dns.TypeA, dns.TypeANY:
			responseMsg = d.localDomain.ResolveAnswer([]string{requestMsg.Question[0].Name}, responseWriter, requestMsg)
		default:
			responseMsg.SetRcode(requestMsg, dns.RcodeServerFailure)
		}
	}

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

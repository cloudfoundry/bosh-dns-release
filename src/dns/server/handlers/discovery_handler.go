package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"dns/server/records/dnsresolver"
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

	if len(requestMsg.Question) > 0 {
		switch requestMsg.Question[0].Qtype {
		case dns.TypeA, dns.TypeANY:
			responseMsg = d.localDomain.Resolve([]string{requestMsg.Question[0].Name}, responseWriter, requestMsg)
		case dns.TypeMX, dns.TypeAAAA:
			responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)
		default:
			responseMsg.SetRcode(requestMsg, dns.RcodeServerFailure)
		}
	}

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

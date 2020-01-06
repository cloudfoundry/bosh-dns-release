package handlers

import (
	"bosh-dns/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger"
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

	if len(requestMsg.Question) > 0 {
		switch requestMsg.Question[0].Qtype {
		case dns.TypeA, dns.TypeANY, dns.TypeAAAA:
			responseMsg = d.localDomain.Resolve([]string{requestMsg.Question[0].Name}, responseWriter, requestMsg)
		case dns.TypeMX:
			responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)
		case dns.TypeSRV:
			responseMsg.SetRcode(requestMsg, dns.RcodeNotImplemented)
		default:
			responseMsg.SetRcode(requestMsg, dns.RcodeServerFailure)
		}
	}

	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = true

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

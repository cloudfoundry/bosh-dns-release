package handlers

import (
	"bosh-dns/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type DiscoveryHandler struct {
	logger             logger.Logger
	logTag             string
	localDomain        dnsresolver.LocalDomain
	recursionAvailable bool
}

func NewDiscoveryHandler(logger logger.Logger, localDomain dnsresolver.LocalDomain, recursionAvailable bool) DiscoveryHandler {
	return DiscoveryHandler{
		logger:             logger,
		logTag:             "DiscoveryHandler",
		localDomain:        localDomain,
		recursionAvailable: recursionAvailable,
	}
}

func (d DiscoveryHandler) ServeDNS(responseWriter dns.ResponseWriter, requestMsg *dns.Msg) {
	responseMsg := &dns.Msg{}

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

	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = d.recursionAvailable

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

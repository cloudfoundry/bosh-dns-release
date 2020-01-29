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
		hostResponse := d.localDomain.Resolve(responseWriter, requestMsg)
		switch requestMsg.Question[0].Qtype {
		case dns.TypeA, dns.TypeANY, dns.TypeAAAA:
			responseMsg = hostResponse
		default:
			if hostResponse.Rcode == dns.RcodeNameError {
				responseMsg.SetRcode(requestMsg, dns.RcodeNameError)
			} else {
				responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)
			}
		}
	}

	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = true

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

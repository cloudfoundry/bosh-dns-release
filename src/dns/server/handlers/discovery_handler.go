package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"net"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
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

func (d DiscoveryHandler) ServeDNS(responseWriter dns.ResponseWriter, req *dns.Msg) {
	responseMsg := &dns.Msg{}
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = false

	if len(req.Question) == 0 {
		responseMsg.SetRcode(req, dns.RcodeSuccess)
	} else {
		switch req.Question[0].Qtype {
		case dns.TypeMX, dns.TypeAAAA:
			responseMsg.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeA, dns.TypeANY:
			responseMsg = d.buildARecords(responseWriter, req)
		default:
			responseMsg.SetRcode(req, dns.RcodeServerFailure)
		}
	}

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

func (d DiscoveryHandler) buildARecords(responseWriter dns.ResponseWriter, requestMsg *dns.Msg) *dns.Msg {
	protocol := dnsresolver.UDP
	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); ok {
		protocol = dnsresolver.TCP
	}

	resolvedAnswer := d.localDomain.ResolveAnswer(requestMsg.Question[0].Name, []string{requestMsg.Question[0].Name}, protocol,requestMsg)

	return resolvedAnswer
}

func (DiscoveryHandler) trimIfNeeded(resp *dns.Msg) {
	numAnswers := len(resp.Answer)

	for len(resp.Answer) > 0 && resp.Len() > 512 {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
	}

	resp.Truncated = len(resp.Answer) < numAnswers
}

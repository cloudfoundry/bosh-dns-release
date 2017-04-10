package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type DiscoveryHandler struct {
	logger logger.Logger
	logTag string
}

func NewDiscoveryHandler(logger logger.Logger) DiscoveryHandler {
	return DiscoveryHandler{
		logger: logger,
		logTag: "DiscoveryHandler",
	}
}

func (d DiscoveryHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	m := &dns.Msg{}

	m.Authoritative = true
	m.RecursionAvailable = false

	if len(req.Question) == 0 {
		m.SetRcode(req, dns.RcodeSuccess)
	} else {
		switch req.Question[0].Qtype {
		case dns.TypeMX:
			m.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeAAAA:
			m.SetRcode(req, dns.RcodeSuccess)
		default:
			m.SetRcode(req, dns.RcodeServerFailure)
		}
	}

	if err := resp.WriteMsg(m); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

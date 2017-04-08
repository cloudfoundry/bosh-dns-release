package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type HealthCheckHandler struct {
	logger logger.Logger
}

func NewHealthCheckHandler(logger logger.Logger) HealthCheckHandler {
	return HealthCheckHandler{
		logger: logger,
	}
}

func (h HealthCheckHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.RecursionAvailable = false
	m.Authoritative = true
	m.SetRcode(req, dns.RcodeSuccess)
	if err := resp.WriteMsg(m); err != nil {
		h.logger.Error("HealthCheckHandler", err.Error())
	}
}

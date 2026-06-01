package handlers

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type DenyHandler struct {
	responseCode int
	logger       boshlog.Logger
}

func NewDenyHandler(responseType string, logger boshlog.Logger) DenyHandler {
	rcode := dns.RcodeNameError // NXDOMAIN by default

	if responseType == "REFUSED" {
		rcode = dns.RcodeRefused
	}

	return DenyHandler{
		responseCode: rcode,
		logger:       logger,
	}
}

func (h DenyHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(r)
	response.SetRcode(r, h.responseCode)
	response.Authoritative = true
	response.RecursionAvailable = false

	if err := w.WriteMsg(response); err != nil {
		h.logger.Error("DenyHandler", "error writing response: %s", err.Error())
	}
}

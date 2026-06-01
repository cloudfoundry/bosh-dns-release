package handlers

import (
	"github.com/miekg/dns"
)

type DenyHandler struct {
	responseCode int
}

func NewDenyHandler(responseType string) DenyHandler {
	rcode := dns.RcodeNameError // NXDOMAIN by default

	if responseType == "REFUSED" {
		rcode = dns.RcodeRefused
	}

	return DenyHandler{
		responseCode: rcode,
	}
}

func (h DenyHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(r)
	response.SetRcode(r, h.responseCode)
	response.Authoritative = true
	response.RecursionAvailable = false

	//nolint:errcheck
	w.WriteMsg(response)
}

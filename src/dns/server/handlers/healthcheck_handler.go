package handlers

import "github.com/miekg/dns"

type HealthCheckHandler struct{}

func NewHealthCheckHandler() HealthCheckHandler {
	return HealthCheckHandler{}
}

func (h HealthCheckHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.RecursionAvailable = false
	m.Authoritative = true
	m.SetRcode(req, dns.RcodeSuccess)
	resp.WriteMsg(m)
}

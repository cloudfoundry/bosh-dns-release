package handlers

import (
	"net"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

var localhostIP = net.ParseIP("127.0.0.1")

type UpcheckHandler struct {
	logger logger.Logger
}

func NewUpcheckHandler(logger logger.Logger) UpcheckHandler {
	return UpcheckHandler{
		logger: logger,
	}
}

func (h UpcheckHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	msg := new(dns.Msg)
	msg.Authoritative = true
	msg.RecursionAvailable = false

	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   req.Question[0].Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		A: localhostIP,
	})
	msg.SetReply(req)
	msg.SetRcode(req, dns.RcodeSuccess)
	if err := resp.WriteMsg(msg); err != nil {
		h.logger.Error("UpcheckHandler", err.Error())
	}
}

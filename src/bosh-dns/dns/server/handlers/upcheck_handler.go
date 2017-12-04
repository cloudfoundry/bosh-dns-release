package handlers

import (
	"net"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

var localhostIP = net.ParseIP("127.0.0.1")
var localhostIPv6 = net.ParseIP("::1")

type UpcheckHandler struct {
	logger logger.Logger
}

func NewUpcheckHandler(logger logger.Logger) UpcheckHandler {
	return UpcheckHandler{
		logger: logger,
	}
}

func (h UpcheckHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	out := &dns.Msg{}
	out.Authoritative = true
	out.RecursionAvailable = true

	if len(req.Question) > 0 {
		reqType := req.Question[0].Qtype

		if reqType == dns.TypeANY || reqType == dns.TypeA {
			out.Answer = append(out.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: localhostIP,
			})
		}

		if reqType == dns.TypeANY || reqType == dns.TypeAAAA {
			out.Answer = append(out.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				AAAA: localhostIPv6,
			})
		}
	}

	out.SetReply(req)
	// rcode is succeess by default

	if err := resp.WriteMsg(out); err != nil {
		h.logger.Error("UpcheckHandler", err.Error())
	}
}

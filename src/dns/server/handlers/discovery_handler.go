package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"net"
)

type DiscoveryHandler struct {
	logger   logger.Logger
	logTag   string
	ipGetter IPGetter
}

//go:generate counterfeiter . IPGetter
type IPGetter interface {
	GetIPs(string) ([]string, error)
}

func NewDiscoveryHandler(logger logger.Logger, ipGetter IPGetter) DiscoveryHandler {
	return DiscoveryHandler{
		logger:   logger,
		logTag:   "DiscoveryHandler",
		ipGetter: ipGetter,
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
		case dns.TypeA:
			d.buildARecords(m, req)
		case dns.TypeANY:
			d.buildARecords(m, req)
		default:
			m.SetRcode(req, dns.RcodeServerFailure)
		}
	}

	if err := resp.WriteMsg(m); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

func (d DiscoveryHandler) buildARecords(msg, req *dns.Msg) {
	records, err := d.ipGetter.GetIPs(req.Question[0].Name)
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		msg.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	if len(records) > 1 {
		d.logger.Info(d.logTag, "got multiple ip addresses for %s: %v", req.Question[0].Name, records)
	}

	for _, r := range records {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(r),
		})
	}

	msg.SetRcode(req, dns.RcodeSuccess)
}

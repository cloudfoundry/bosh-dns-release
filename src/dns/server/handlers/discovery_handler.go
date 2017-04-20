package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"
	"net"
)

type DiscoveryHandler struct {
	logger        logger.Logger
	logTag        string
	recordSetRepo RecordSetRepo
}

//go:generate counterfeiter . RecordSetRepo
type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

func NewDiscoveryHandler(logger logger.Logger, recordSetRepo RecordSetRepo) DiscoveryHandler {
	return DiscoveryHandler{
		logger:        logger,
		logTag:        "DiscoveryHandler",
		recordSetRepo: recordSetRepo,
	}
}

func (d DiscoveryHandler) ServeDNS(responseWriter dns.ResponseWriter, req *dns.Msg) {
	response := &dns.Msg{}

	response.Authoritative = true
	response.RecursionAvailable = false

	if len(req.Question) == 0 {
		response.SetRcode(req, dns.RcodeSuccess)
	} else {
		switch req.Question[0].Qtype {
		case dns.TypeMX:
			response.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeAAAA:
			response.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeA:
			d.buildARecords(response, req)
		case dns.TypeANY:
			d.buildARecords(response, req)
		default:
			response.SetRcode(req, dns.RcodeServerFailure)
		}
	}

	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); !ok {
		d.trimIfNeeded(response)
	}

	if err := responseWriter.WriteMsg(response); err != nil {
		d.logger.Error(d.logTag, err.Error())
	}
}

func (DiscoveryHandler) trimIfNeeded(resp *dns.Msg) {
	numAnswers := len(resp.Answer)
	for len(resp.Answer) > 0 && resp.Len() > 512 {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
	}
	resp.Truncated = len(resp.Answer) < numAnswers
}

func (d DiscoveryHandler) buildARecords(msg, req *dns.Msg) {
	recordSet, err := d.recordSetRepo.Get()
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		msg.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	records, err := recordSet.Resolve(req.Question[0].Name)
	if err != nil {
		d.logger.Error(d.logTag, "failed to decode query: %v", err)
		msg.SetRcode(req, dns.RcodeFormatError)
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

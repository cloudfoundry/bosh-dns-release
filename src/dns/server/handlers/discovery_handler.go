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
	shuffler      Shuffler
}

//go:generate counterfeiter . Shuffler
type Shuffler interface {
	Shuffle(src []string) []string
}

//go:generate counterfeiter . RecordSetRepo
type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

func NewDiscoveryHandler(logger logger.Logger, shuffler Shuffler, recordSetRepo RecordSetRepo) DiscoveryHandler {
	return DiscoveryHandler{
		logger:        logger,
		logTag:        "DiscoveryHandler",
		recordSetRepo: recordSetRepo,
		shuffler:      shuffler,
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
		case dns.TypeMX:
			responseMsg.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeAAAA:
			responseMsg.SetRcode(req, dns.RcodeSuccess)
		case dns.TypeA, dns.TypeANY:
			d.buildARecords(responseWriter, responseMsg, req)
		default:
			responseMsg.SetRcode(req, dns.RcodeServerFailure)
		}
	}

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
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

func (d DiscoveryHandler) buildARecords(responseWriter dns.ResponseWriter, responseMsg, requestMsg *dns.Msg) {
	recordSet, err := d.recordSetRepo.Get()
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		responseMsg.SetRcode(requestMsg, dns.RcodeServerFailure)
		return
	}

	ips, err := recordSet.Resolve(requestMsg.Question[0].Name)
	if err != nil {
		d.logger.Error(d.logTag, "failed to decode query: %v", err)
		responseMsg.SetRcode(requestMsg, dns.RcodeFormatError)
		return
	}

	ips = d.shuffler.Shuffle(ips)

	for _, ip := range ips {
		responseMsg.Answer = append(responseMsg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   requestMsg.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(ip),
		})
	}

	responseMsg.SetRcode(requestMsg, dns.RcodeSuccess)

	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); !ok {
		d.trimIfNeeded(responseMsg)
	}
}

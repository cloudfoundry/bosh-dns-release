package handlers

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"net"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
)

type DiscoveryHandler struct {
	logger   logger.Logger
	logTag   string
	shuffler AnswerShuffler
	answerer dnsresolver.LocalDomain
}

//go:generate counterfeiter . AnswerShuffler
type AnswerShuffler interface {
	Shuffle(src []dns.RR) []dns.RR
}

func NewDiscoveryHandler(logger logger.Logger, shuffler AnswerShuffler, answerer dnsresolver.LocalDomain) DiscoveryHandler {
	return DiscoveryHandler{
		logger:   logger,
		logTag:   "DiscoveryHandler",
		shuffler: shuffler,
		answerer: answerer,
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

func (d DiscoveryHandler) buildARecords(responseWriter dns.ResponseWriter, responseMsg, requestMsg *dns.Msg) {
	answer, rcode := d.answerer.Resolve(requestMsg.Question[0].Name, requestMsg.Question[0].Name)
	responseMsg.SetRcode(requestMsg, rcode)
	responseMsg.Answer = d.shuffler.Shuffle(answer)

	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); !ok {
		d.trimIfNeeded(responseMsg)
	}
}

func (DiscoveryHandler) trimIfNeeded(resp *dns.Msg) {
	numAnswers := len(resp.Answer)
	for len(resp.Answer) > 0 && resp.Len() > 512 {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
	}
	resp.Truncated = len(resp.Answer) < numAnswers
}

package dnsresolver

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"
	"net"
)

type LocalDomain struct {
	logger        logger.Logger
	logTag        string
	recordSetRepo RecordSetRepo
	shuffler      AnswerShuffler
}

type Protocol int

const (
	UDP Protocol = iota
	TCP
)

//go:generate counterfeiter . RecordSetRepo
type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

//go:generate counterfeiter . AnswerShuffler
type AnswerShuffler interface {
	Shuffle(src []dns.RR) []dns.RR
}

func NewLocalDomain(logger logger.Logger, recordSetRepo RecordSetRepo, shuffler AnswerShuffler) LocalDomain {
	return LocalDomain{
		logger:        logger,
		logTag:        "LocalDomain",
		recordSetRepo: recordSetRepo,
		shuffler:      shuffler,
	}
}

func (d LocalDomain) ResolveAnswer(answerDomain string, questionDomains []string, protocol Protocol, requestMsg *dns.Msg) *dns.Msg {
	answers, rCode := d.resolve(requestMsg.Question[0].Name, questionDomains)
	responseMsg := &dns.Msg{}
	responseMsg.Answer = answers
	responseMsg.SetRcode(requestMsg, rCode)
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = false

	d.trimIfNeeded(protocol, responseMsg)

	return responseMsg
}

func (d LocalDomain) resolve(answerDomain string, questionDomains []string) ([]dns.RR, int) {
	recordSet, err := d.recordSetRepo.Get()
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		return nil, dns.RcodeServerFailure
	}

	answers := []dns.RR{}

	for _, questionDomain := range questionDomains {
		ips, err := recordSet.Resolve(questionDomain)
		if err != nil {
			d.logger.Error(d.logTag, "failed to decode query: %v", err)
			return nil, dns.RcodeFormatError
		}

		for _, ip := range ips {
			answers = append(answers, &dns.A{
				Hdr: dns.RR_Header{
					Name:   answerDomain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(ip),
			})
		}
	}

	return d.shuffler.Shuffle(answers), dns.RcodeSuccess
}

func (LocalDomain) trimIfNeeded(protocol Protocol, resp *dns.Msg) {
		if protocol != UDP {
			return
		}

		numAnswers := len(resp.Answer)

		for len(resp.Answer) > 0 && resp.Len() > 512 {
			resp.Answer = resp.Answer[:len(resp.Answer)-1]
		}

		resp.Truncated = len(resp.Answer) < numAnswers
}

package dnsresolver

import (
	"net"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type LocalDomain struct {
	logger    logger.Logger
	logTag    string
	recordSet RecordSet
	shuffler  AnswerShuffler
}

//go:generate counterfeiter . AnswerShuffler

type AnswerShuffler interface {
	Shuffle(src []dns.RR) []dns.RR
}

//go:generate counterfeiter . RecordSet

type RecordSet interface {
	Resolve(domain string) ([]string, error)
}

func NewLocalDomain(logger logger.Logger, recordSet RecordSet, shuffler AnswerShuffler) LocalDomain {
	return LocalDomain{
		logger:    logger,
		logTag:    "LocalDomain",
		recordSet: recordSet,
		shuffler:  shuffler,
	}
}

func (d LocalDomain) Resolve(questionDomains []string, responseWriter dns.ResponseWriter, requestMsg *dns.Msg) *dns.Msg {
	answers, rCode := d.resolve(requestMsg.Question[0], questionDomains)

	responseMsg := &dns.Msg{}
	responseMsg.RecursionAvailable = true
	responseMsg.Authoritative = true
	responseMsg.Answer = answers
	responseMsg.SetRcode(requestMsg, rCode)

	TruncateIfNeeded(responseWriter, responseMsg)

	return responseMsg
}

func (d LocalDomain) resolve(question dns.Question, questionDomains []string) ([]dns.RR, int) {
	answers := []dns.RR{}

	for _, questionDomain := range questionDomains {
		ipStrs, err := d.recordSet.Resolve(questionDomain)
		if err != nil {
			d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
			return nil, dns.RcodeFormatError
		}

		for _, ipStr := range ipStrs {
			var answer dns.RR

			ip := net.ParseIP(ipStr)

			if ip.To4() != nil {
				if question.Qtype == dns.TypeA || question.Qtype == dns.TypeANY {
					answer = &dns.A{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						A: ip,
					}
				}
			} else {
				if question.Qtype == dns.TypeAAAA || question.Qtype == dns.TypeANY {
					answer = &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						AAAA: ip,
					}
				}
			}

			if answer != nil {
				answers = append(answers, answer)
			}
		}
	}

	return d.shuffler.Shuffle(answers), dns.RcodeSuccess
}

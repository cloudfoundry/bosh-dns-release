package dnsresolver

import (
	"bosh-dns/dns/server/records"
	"errors"
	"net"
	"strings"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type LocalDomain struct {
	logger    logger.Logger
	logTag    string
	recordSet RecordSet
	shuffler  AnswerShuffler
	truncater ResponseTruncater
}

//counterfeiter:generate . AnswerShuffler

type AnswerShuffler interface {
	Shuffle(src []dns.RR) []dns.RR
}

//counterfeiter:generate . RecordSet

type RecordSet interface {
	Resolve(domain string) ([]string, error)
}

func NewLocalDomain(logger logger.Logger, recordSet RecordSet, shuffler AnswerShuffler, truncater ResponseTruncater) LocalDomain {
	return LocalDomain{
		logger:    logger,
		logTag:    "LocalDomain",
		recordSet: recordSet,
		shuffler:  shuffler,
		truncater: truncater,
	}
}

func (d LocalDomain) Resolve(responseWriter dns.ResponseWriter, requestMsg *dns.Msg) *dns.Msg {
	answers, rCode := d.resolve(requestMsg.Question[0])

	responseMsg := &dns.Msg{}
	responseMsg.RecursionAvailable = true
	responseMsg.Authoritative = true
	responseMsg.Answer = answers
	responseMsg.SetRcode(requestMsg, rCode)

	d.truncater.TruncateIfNeeded(responseWriter, requestMsg, responseMsg)

	return responseMsg
}

func (d LocalDomain) resolve(question dns.Question) ([]dns.RR, int) {
	var lowercaseName = strings.ToLower(question.Name)

	d.logger.Debug(d.logTag, "query lower-cased from '%s' to '%s'", question.Name, lowercaseName)

	answers := []dns.RR{}

	ipStrs, err := d.recordSet.Resolve(lowercaseName)
	if err != nil {
		d.logger.Debug(d.logTag, "failed to get ip addresses: %v", err)
		if errors.Is(err, records.CriteriaError) {
			return nil, dns.RcodeFormatError
		} else if errors.Is(err, records.DomainError) {
			return nil, dns.RcodeNameError
		} else {
			return nil, dns.RcodeServerFailure
		}
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

	return d.shuffler.Shuffle(answers), dns.RcodeSuccess
}

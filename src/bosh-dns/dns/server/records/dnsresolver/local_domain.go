package dnsresolver

import (
	"net"

	"bosh-dns/dns/server/records"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type LocalDomain struct {
	logger        logger.Logger
	logTag        string
	recordSetRepo RecordSetRepo
	shuffler      AnswerShuffler
	healthLookup  HealthLookup
}

//go:generate counterfeiter . RecordSetRepo
type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

//go:generate counterfeiter . AnswerShuffler
type AnswerShuffler interface {
	Shuffle(src []dns.RR) []dns.RR
}

//go:generate counterfeiter . HealthLookup
type HealthLookup interface {
	IsHealthy(ip string) bool
}

func NewLocalDomain(logger logger.Logger, recordSetRepo RecordSetRepo, shuffler AnswerShuffler, healthLookup HealthLookup) LocalDomain {
	return LocalDomain{
		logger:        logger,
		logTag:        "LocalDomain",
		recordSetRepo: recordSetRepo,
		shuffler:      shuffler,
		healthLookup:  healthLookup,
	}
}

func (d LocalDomain) Resolve(questionDomains []string, responseWriter dns.ResponseWriter, requestMsg *dns.Msg) *dns.Msg {
	answers, rCode := d.resolve(requestMsg.Question[0].Name, questionDomains)

	responseMsg := &dns.Msg{}
	responseMsg.Answer = answers
	responseMsg.SetRcode(requestMsg, rCode)
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = false

	d.trimIfNeeded(responseWriter, responseMsg)

	return responseMsg
}

func (d LocalDomain) resolve(answerDomain string, questionDomains []string) ([]dns.RR, int) {
	recordSet, err := d.recordSetRepo.Get()
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		return nil, dns.RcodeServerFailure
	}

	answers := []dns.RR{}
	healthyAnswers := []dns.RR{}

	for _, questionDomain := range questionDomains {
		ips, err := recordSet.Resolve(questionDomain)
		if err != nil {
			d.logger.Error(d.logTag, "failed to decode query: %v", err)
			return nil, dns.RcodeFormatError
		}

		for _, ip := range ips {
			answer := &dns.A{
				Hdr: dns.RR_Header{
					Name:   answerDomain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(ip),
			}

			if d.healthLookup.IsHealthy(ip) {
				healthyAnswers = append(healthyAnswers, answer)
			} else {
				answers = append(answers, answer)
			}
		}
	}

	if len(healthyAnswers) > 0 {
		answers = healthyAnswers
	} else {
		d.logger.Info(d.logTag, "No healthy IP addresses found, returning all IP addresses")
	}
	return d.shuffler.Shuffle(answers), dns.RcodeSuccess
}

func (LocalDomain) trimIfNeeded(responseWriter dns.ResponseWriter, resp *dns.Msg) {
	maxLength := dns.MaxMsgSize
	_, isUDP := responseWriter.RemoteAddr().(*net.UDPAddr)

	if isUDP {
		maxLength = 512
	}

	numAnswers := len(resp.Answer)

	for len(resp.Answer) > 0 && resp.Len() > maxLength {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
	}

	resp.Truncated = isUDP && len(resp.Answer) < numAnswers
}

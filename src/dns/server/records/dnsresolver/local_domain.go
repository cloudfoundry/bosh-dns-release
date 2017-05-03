package dnsresolver

import (
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"net"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
)

type LocalDomain struct {
	logger        logger.Logger
	logTag        string
	recordSetRepo RecordSetRepo
}

//go:generate counterfeiter . RecordSetRepo
type RecordSetRepo interface {
	Get() (records.RecordSet, error)
}

func NewLocalDomain(logger logger.Logger, recordSetRepo RecordSetRepo) LocalDomain {
	return LocalDomain{
		logger:        logger,
		logTag:        "LocalDomain",
		recordSetRepo: recordSetRepo,
	}
}

func (d LocalDomain) Resolve(answerName, resolutionName string) ([]dns.RR, int) {
	recordSet, err := d.recordSetRepo.Get()
	if err != nil {
		d.logger.Error(d.logTag, "failed to get ip addresses: %v", err)
		return nil, dns.RcodeServerFailure
	}

	ips, err := recordSet.Resolve(resolutionName)
	if err != nil {
		d.logger.Error(d.logTag, "failed to decode query: %v", err)
		return nil, dns.RcodeFormatError
	}

	answer := []dns.RR{}

	for _, ip := range ips {
		answer = append(answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   answerName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(ip),
		})
	}

	return answer, dns.RcodeSuccess
}

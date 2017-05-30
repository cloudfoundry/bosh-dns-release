package server

import (
	"errors"
	"net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/miekg/dns"
)

type Dialer func(string, string) (net.Conn, error)

//go:generate counterfeiter . HealthCheck
type HealthCheck interface {
	IsHealthy() error
}

type AnswerValidatingHealthCheck struct {
	target            string
	healthCheckDomain string
	network           string
}

func NewAnswerValidatingHealthCheck(target string, healthcheckDomain string, network string) HealthCheck {
	return AnswerValidatingHealthCheck{
		target:            target,
		healthCheckDomain: healthcheckDomain,
		network:           network,
	}
}

func (hc AnswerValidatingHealthCheck) IsHealthy() error {
	var err error
	hc.target, err = determineHost(hc.target)
	if err != nil {
		return hc.wrapError(err)
	}

	dnsClient := dns.Client{Net: hc.network}
	request := dns.Msg{
		Question: []dns.Question{
			{Name: hc.healthCheckDomain},
		},
	}
	msg, _, err := dnsClient.Exchange(&request, hc.target)

	if err != nil {
		return hc.wrapError(err)
	}
	if msg.Rcode != dns.RcodeSuccess {
		return hc.wrapError(errors.New("DNS reolve failed"))
	}

	if len(msg.Answer) == 0 {
		return hc.wrapError(errors.New("DNS healthcheck found no answers"))
	}

	aRecord, ok := msg.Answer[0].(*dns.A)
	if !ok {
		return hc.wrapError(errors.New("health check must return A record"))
	}

	if !aRecord.A.Equal(net.ParseIP("127.0.0.1")) {
		return hc.wrapError(errors.New("DNS healthcheck does not return the correct answer"))
	}

	return nil
}

func determineHost(target string) (string, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", err
	}

	if host == "0.0.0.0" {
		return net.JoinHostPort("127.0.0.1", port), nil
	}

	return target, nil
}

func (h AnswerValidatingHealthCheck) wrapError(err error) error {
	return bosherr.WrapErrorf(err, "on %s", h.network)
}

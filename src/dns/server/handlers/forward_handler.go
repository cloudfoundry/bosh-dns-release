package handlers

import (
	"net"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type ForwardHandler struct {
	recursors        []string
	exchangerFactory ExchangerFactory
	logger           logger.Logger
	logTag           string
}

//go:generate counterfeiter . Exchanger
type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

func NewForwardHandler(recursors []string, exchangerFactory ExchangerFactory, logger logger.Logger) ForwardHandler {
	return ForwardHandler{
		recursors:        recursors,
		exchangerFactory: exchangerFactory,
		logger:           logger,
		logTag:           "ForwardHandler",
	}
}

func (r ForwardHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	m := &dns.Msg{}

	if len(req.Question) == 0 {
		r.logger.Info(r.logTag, "received a request with no questions")
		m.RecursionAvailable = false
		m.Authoritative = true
		m.SetRcode(req, dns.RcodeSuccess)
		if err := resp.WriteMsg(m); err != nil {
			r.logger.Error(r.logTag, "error writing response %s", err.Error())
		}
		return
	}

	network := "udp"
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	client := r.exchangerFactory(network)
	r.logger.Info(r.logTag, "attempting recursors")
	for _, recursor := range r.recursors {
		answer, _, err := client.Exchange(req, recursor)
		if err == nil || err == dns.ErrTruncated {
			if err := resp.WriteMsg(answer); err != nil {
				r.logger.Error(r.logTag, "error writing response %s", err.Error())
			}
			return
		}
		r.logger.Info(r.logTag, "error recursing to %s %s", recursor, err.Error())
	}

	r.logger.Info(r.logTag, "no response from recursors")

	m.SetReply(req)
	m.RecursionAvailable = true
	m.Authoritative = false
	m.SetRcode(req, dns.RcodeServerFailure)
	if err := resp.WriteMsg(m); err != nil {
		r.logger.Error(r.logTag, "error writing response %s", err.Error())
	}
}

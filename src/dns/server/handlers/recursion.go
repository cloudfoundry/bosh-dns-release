package handlers

import (
	"net"
	"time"

	"github.com/miekg/dns"
)

type Recursion struct {
	recursors        []string
	exchangerFactory ExchangerFactory
}

//go:generate counterfeiter . Exchanger
type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

type ExchangerFactory func(string) Exchanger

func NewRecursion(recursors []string, exchangerFactory ExchangerFactory) Recursion {
	return Recursion{
		recursors:        recursors,
		exchangerFactory: exchangerFactory,
	}
}

func (r Recursion) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)

	if len(req.Question) == 0 {
		m.RecursionAvailable = false
		m.Authoritative = true
		m.SetRcode(req, dns.RcodeSuccess)
		resp.WriteMsg(m)
		return
	}

	network := "udp"
	if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	client := r.exchangerFactory(network)

	for _, recursor := range r.recursors {
		answer, _, err := client.Exchange(req, recursor)
		if err == nil {
			resp.WriteMsg(answer)
			return
		}
	}

	m.SetReply(req)
	m.RecursionAvailable = true
	m.Authoritative = false
	m.SetRcode(req, dns.RcodeServerFailure)
	resp.WriteMsg(m)
}

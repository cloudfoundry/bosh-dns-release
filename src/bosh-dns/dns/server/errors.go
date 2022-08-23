package server

import (
	"fmt"

	"github.com/miekg/dns"
)

type DnsError interface {
	error
	Rcode() int
	Question() string
	Recursor() string
}

type dnsError struct {
	rcode    int
	question string
	recursor string
}

func (e *dnsError) Rcode() int {
	return e.rcode
}

func (e *dnsError) Question() string {
	return e.question
}

func (e *dnsError) Recursor() string {
	return e.recursor
}

func (e *dnsError) Error() string {
	return fmt.Sprintf(
		"received %s for %s from upstream (recursor: %s)",
		dns.RcodeToString[e.Rcode()],
		e.Question(),
		e.Recursor(),
	)
}

func NewDnsError(rcode int, question string, recursor string) DnsError {
	return &dnsError{rcode, question, recursor}
}

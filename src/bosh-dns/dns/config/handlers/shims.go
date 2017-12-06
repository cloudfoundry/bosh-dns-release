package handlers

import "github.com/miekg/dns"

//go:generate counterfeiter . dnsHandler

type dnsHandler interface {
	dns.Handler
}

package handlers

import "github.com/miekg/dns"

//go:generate counterfeiter . dnsHandler

type dnsHandler interface { //nolint:deadcode,unused
	dns.Handler
}

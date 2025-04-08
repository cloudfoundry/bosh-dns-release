package handlers

import "github.com/miekg/dns"

//counterfeiter:generate . dnsHandler

type dnsHandler interface { //nolint:unused
	dns.Handler
}

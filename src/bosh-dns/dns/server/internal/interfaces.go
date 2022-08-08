package internal

import "github.com/miekg/dns"

//go:generate counterfeiter net.Conn

//go:generate counterfeiter . responseWriter

type responseWriter interface { //nolint:deadcode,unused
	dns.ResponseWriter
}

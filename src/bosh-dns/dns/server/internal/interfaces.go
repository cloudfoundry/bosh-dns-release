package internal

import "github.com/miekg/dns"

//go:generate counterfeiter net.Conn

//go:generate counterfeiter . responseWriter

type responseWriter interface {
	dns.ResponseWriter
}

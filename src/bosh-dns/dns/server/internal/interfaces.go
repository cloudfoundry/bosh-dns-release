package internal

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import "github.com/miekg/dns"

//counterfeiter:generate net.Conn

//counterfeiter:generate . responseWriter

type responseWriter interface { //nolint:unused
	dns.ResponseWriter
}

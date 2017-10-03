package dnsresolver

import (
	"github.com/miekg/dns"
	"net"
)

func TruncateIfNeeded(responseWriter dns.ResponseWriter, resp *dns.Msg) {
	maxLength := dns.MaxMsgSize
	_, isUDP := responseWriter.RemoteAddr().(*net.UDPAddr)

	if isUDP {
		maxLength = 512
	}

	numAnswers := len(resp.Answer)

	for len(resp.Answer) > 0 && resp.Len() > maxLength {
		resp.Answer = resp.Answer[:len(resp.Answer)-1]
	}

	resp.Truncated = (isUDP && len(resp.Answer) < numAnswers) || resp.Truncated
}
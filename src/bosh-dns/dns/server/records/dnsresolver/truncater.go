package dnsresolver

import (
	"net"

	"github.com/miekg/dns"
)

//go:generate counterfeiter . ResponseTruncater
type ResponseTruncater interface {
	TruncateIfNeeded(responseWriter dns.ResponseWriter, req, resp *dns.Msg)
}

type truncater struct{}

func NewResponseTruncater() ResponseTruncater {
	return &truncater{}
}

func (t *truncater) TruncateIfNeeded(responseWriter dns.ResponseWriter, req, resp *dns.Msg) {
	maxLength := dns.MaxMsgSize
	_, isUDP := responseWriter.RemoteAddr().(*net.UDPAddr)

	if isUDP {
		reqEdns := req.IsEdns0()
		if reqEdns != nil {
			maxLength = int(reqEdns.UDPSize())
			if resp.IsEdns0() == nil {
				resp.SetEdns0(uint16(maxLength), false)
			}
		} else {
			maxLength = dns.MinMsgSize
		}
	}
	if resp.Truncated && resp.Len() < maxLength {
		return
	}
	upstreamTruncated := resp.Truncated
	resp.Truncate(maxLength)
	if upstreamTruncated {
		resp.Truncated = true // resp.Truncate clears truncation flag if it didn't remove any records
	}
}

package helpers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/gomega"
)

type DigOpts struct {
	BufferSize     uint16
	Port           int
	SkipRcodeCheck bool
	SkipErrCheck   bool
	Timeout        time.Duration
}

func Dig(domain, server string) *dns.Msg {
	return DigWithPort(domain, server, 53)
}

func DigWithPort(domain, server string, port int) *dns.Msg {
	r := DigWithOptions(domain, server, DigOpts{Port: port})
	Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
	return r
}

func ReverseDigWithOptions(domain, server string, opts DigOpts) *dns.Msg {
	var reversedOctets []string
	octets := strings.Split(domain, ".")
	for _, v := range octets {
		reversedOctets = append([]string{v}, reversedOctets...)
	}
	reversedAddress := strings.Join([]string(reversedOctets), ".")
	reversedAddress += ".in-addr.arpa."
	return DigWithOptions(reversedAddress, server, opts)
}

func ReverseDig(domain, server string) *dns.Msg {
	return ReverseDigWithOptions(domain, server, DigOpts{})
}

func IPv6ReverseDig(domain, server string) *dns.Msg {
	return IPv6ReverseDigWithOptions(domain, server, DigOpts{})
}

func IPv6ReverseDigWithOptions(domain, server string, opts DigOpts) *dns.Msg {
	expandedAddress := net.ParseIP(domain)
	octets := []string{}
	for _, v := range expandedAddress.To16() {
		octets = append(octets, fmt.Sprintf("%02x", v))
	}

	reversedOctets := []string{}
	for _, v := range strings.Join(octets, "") {
		reversedOctets = append([]string{string(v)}, reversedOctets...)
	}
	reversedAddress := strings.Join(reversedOctets, ".")
	reversedAddress += ".ip6.arpa."
	return DigWithOptions(reversedAddress, server, opts)
}

func DigWithOptions(domain, server string, opts DigOpts) *dns.Msg {
	c := &dns.Client{Timeout: opts.Timeout, UDPSize: opts.BufferSize}
	m := &dns.Msg{}
	if opts.BufferSize > dns.MinMsgSize {
		m.SetEdns0(opts.BufferSize, false)
	}
	m.SetQuestion(domain, dns.TypeA)

	port := 53
	if opts.Port != 0 {
		port = opts.Port
	}

	r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", server, port))
	if !opts.SkipErrCheck {
		Expect(err).NotTo(HaveOccurred())
	}

	if !opts.SkipRcodeCheck {
		Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
	}

	return r
}

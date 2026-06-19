package helpers

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/gomega" //nolint:staticcheck
)

type DigOpts struct {
	BufferSize     uint16
	Port           int
	SkipRcodeCheck bool
	SkipErrCheck   bool
	Timeout        time.Duration
	Type           uint16
	Id             uint16
}

// RemoteDig resolves domain from within the given BOSH instance using the VM's
// system resolver. This exercises the full production DNS routing:
//   - Jammy: /etc/resolv.conf → 169.254.0.2 (bosh-dns loopback alias)
//   - Noble and beyond: systemd-resolved stub → routes BOSH domains to bosh-dns at 169.254.0.2
func RemoteDig(instanceSlug, domain string) *dns.Msg {
	return digSSH(instanceSlug, domain, fmt.Sprintf("dig +notcp %s", domain))
}

func digSSH(instanceSlug, domain, digCmd string) *dns.Msg {
	Expect(safeDomainRe.MatchString(domain)).To(BeTrue(),
		"domain %q contains characters unsafe for shell interpolation", domain)
	//nolint:gosec
	cmd := exec.Command(boshBinaryPath, "-n", "ssh", instanceSlug, "-c", digCmd)
	out, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred(),
		"bosh ssh dig failed for instance %s domain %s", instanceSlug, domain)
	msg := parseDigOutput(string(out))
	Expect(msg.Rcode).To(Equal(dns.RcodeSuccess),
		"dig returned non-success rcode for domain %s", domain)
	return msg
}

var (
	digFlagsRe   = regexp.MustCompile(`flags:\s+([\w\s]+);`)
	digAnswerRe  = regexp.MustCompile(`\b(\d+)\s+IN\s+A\s+((?:\d{1,3}\.){3}\d{1,3})`)
	digStatusRe  = regexp.MustCompile(`status:\s+(\w+)`)
	safeDomainRe = regexp.MustCompile(`^[a-zA-Z0-9*._-]+$`)
)

var rcodeMap = map[string]int{
	"NOERROR":  dns.RcodeSuccess,
	"FORMERR":  dns.RcodeFormatError,
	"SERVFAIL": dns.RcodeServerFailure,
	"NXDOMAIN": dns.RcodeNameError,
	"NOTIMP":   dns.RcodeNotImplemented,
	"REFUSED":  dns.RcodeRefused,
}

// parseDigOutput converts dig text output (including the "instance: stdout | ..."
// prefix that bosh ssh adds) into a *dns.Msg suitable for use with gomegadns matchers.
func parseDigOutput(output string) *dns.Msg {
	msg := &dns.Msg{}

	if m := digStatusRe.FindStringSubmatch(output); len(m) > 1 {
		if rcode, ok := rcodeMap[m[1]]; ok {
			msg.Rcode = rcode
		}
	}

	if m := digFlagsRe.FindStringSubmatch(output); len(m) > 1 {
		for _, f := range strings.Fields(m[1]) {
			switch f {
			case "qr":
				msg.Response = true
			case "aa":
				msg.Authoritative = true
			case "tc":
				msg.Truncated = true
			case "rd":
				msg.RecursionDesired = true
			case "ra":
				msg.RecursionAvailable = true
			case "ad":
				msg.AuthenticatedData = true
			case "cd":
				msg.CheckingDisabled = true
			}
		}
	}

	for _, m := range digAnswerRe.FindAllStringSubmatch(output, -1) {
		ttl, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			continue
		}
		ip := net.ParseIP(m[2])
		if ip == nil {
			continue
		}
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   "unknown.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl),
			},
			A: ip,
		})
	}

	return msg
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
	opts.Type = dns.TypePTR
	return DigWithOptions(reversedAddress, server, opts)
}

func ReverseDig(domain, server string) *dns.Msg {
	return ReverseDigWithOptions(domain, server, DigOpts{Type: dns.TypePTR})
}

func IPv6ReverseDig(domain, server string) *dns.Msg {
	return IPv6ReverseDigWithOptions(domain, server, DigOpts{Type: dns.TypePTR})
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
	opts.Type = dns.TypePTR
	return DigWithOptions(reversedAddress, server, opts)
}

func DigWithOptions(domain, server string, opts DigOpts) *dns.Msg {
	c := &dns.Client{Timeout: opts.Timeout, UDPSize: opts.BufferSize}
	m := &dns.Msg{}
	if opts.BufferSize > dns.MinMsgSize {
		m.SetEdns0(opts.BufferSize, false)
	}

	if opts.Type == dns.TypeNone {
		opts.Type = dns.TypeA
	}
	m.SetQuestion(domain, opts.Type)

	if opts.Id != 0 {
		m.Id = opts.Id
	}

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

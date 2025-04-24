package handlers

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

//counterfeiter:generate . IPProvider
//counterfeiter:generate . DNSHandler

type DNSHandler interface {
	dns.Handler
}

type IPProvider interface {
	HasIP(string) bool
	GetFQDNs(string) []string
}

type ArpaHandler struct {
	logger         logger.Logger
	ipProvider     IPProvider
	forwardHandler DNSHandler
	logTag         string
}

func NewArpaHandler(logger logger.Logger, i IPProvider, h DNSHandler) ArpaHandler {
	return ArpaHandler{
		logger:         logger,
		forwardHandler: h,
		ipProvider:     i,
		logTag:         "ArpaHandler",
	}
}

func reverse(ss []string) []string {
	r := make([]string, len(ss))
	for i, s := range ss {
		r[len(ss)-i-1] = s
	}
	return r
}

func (a ArpaHandler) convertToRecordIP(q string) (string, error) {
	var query = strings.ToLower(q)
	a.logger.Debug("ArpaHandler", "query lower-cased from '%s' to '%s'", q, query)

	if strings.HasSuffix(query, ".ip6.arpa.") {
		segments := strings.Split(strings.TrimRight(query, ".ip6.arpa."), ".") //nolint:staticcheck
		reversedSegments := reverse(segments)
		for len(reversedSegments) < 32 {
			reversedSegments = append(reversedSegments, "0")
		}
		response := ""
		for i := 0; i < len(reversedSegments); i++ {
			response += reversedSegments[i]
			if i%4 == 3 && i < len(reversedSegments)-1 {
				response += ":"
			}
		}
		return response, nil
	} else if strings.HasSuffix(query, ".in-addr.arpa.") {
		segments := strings.Split(strings.TrimRight(query, ".in-addr.arpa."), ".") //nolint:staticcheck
		return strings.Join(reverse(segments), "."), nil
	}
	return "", fmt.Errorf("Error converting record '%s' to IP", query) //nolint:staticcheck
}

func (a ArpaHandler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := &dns.Msg{}

	m.Authoritative = true
	m.RecursionAvailable = false
	m.SetReply(req)
	if len(req.Question) == 0 {
		m.SetRcode(req, dns.RcodeSuccess)
		a.logErrors(w, w.WriteMsg(m))
		return
	}

	ip, err := a.convertToRecordIP(req.Question[0].Name)
	if err != nil {
		a.logger.Debug(a.logTag, err.Error())
		m.SetRcode(req, dns.RcodeFormatError)
		a.logErrors(w, w.WriteMsg(m))
		return
	}

	fqdns := a.ipProvider.GetFQDNs(ip)
	if len(fqdns) > 0 {
		m.SetRcode(req, dns.RcodeSuccess)
		for _, fqdn := range fqdns {
			m.Answer = append(m.Answer, &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				Ptr: fqdn,
			})
		}

		a.logErrors(w, w.WriteMsg(m))
		return
	}

	a.forwardHandler.ServeDNS(w, req)
}

func (a ArpaHandler) logErrors(w dns.ResponseWriter, err error) {
	if err != nil {
		a.logger.Error(a.logTag, err.Error())
	}
}

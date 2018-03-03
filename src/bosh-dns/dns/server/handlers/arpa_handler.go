package handlers

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

//go:generate counterfeiter . IPProvider
//go:generate counterfeiter . DNSHandler

type DNSHandler interface {
	dns.Handler
}

type IPProvider interface {
	AllIPs() map[string]bool
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

func convertToRecordIP(query string) (string, error) {
	if strings.HasSuffix(query, ".ip6.arpa.") {
		segments := strings.Split(strings.TrimRight(query, ".ip6.arpa."), ".")
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
		segments := strings.Split(strings.TrimRight(query, ".in-addr.arpa."), ".")
		return strings.Join(reverse(segments), "."), nil
	}
	return "", fmt.Errorf("Error converting record '%s' to IP", query)
}

func (a ArpaHandler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := &dns.Msg{}

	m.Authoritative = true
	m.RecursionAvailable = false
	if len(req.Question) == 0 {
		m.SetRcode(req, dns.RcodeSuccess)
		a.logErrors(w, w.WriteMsg(m))
		return
	}

	ip, err := convertToRecordIP(req.Question[0].Name)
	a.logErrors(w, err)
	a.logger.Info(a.logTag, "received a request with %d questions", len(req.Question))

	if err != nil || a.ipProvider.AllIPs()[ip] {
		m.SetRcode(req, dns.RcodeServerFailure)
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

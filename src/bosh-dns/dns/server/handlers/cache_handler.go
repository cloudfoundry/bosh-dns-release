package handlers

import (
	"bosh-dns/dns/server/handlers/internal"
	"bosh-dns/dns/server/records/dnsresolver"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/miekg/dns"
	"golang.org/x/net/context"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type CachingDNSHandler struct {
	next      dns.Handler
	ca        *cache.Cache
	logger    boshlog.Logger
	logTag    string
	truncater dnsresolver.ResponseTruncater
}

func NewCachingDNSHandler(next dns.Handler, truncater dnsresolver.ResponseTruncater) CachingDNSHandler {
	ca := cache.New()
	ca.Next = corednsHandlerWrapper{Next: next}
	ca.Zones = []string{"."}
	return CachingDNSHandler{
		ca:        ca,
		logTag:    "CachingDNSHandler",
		next:      next,
		truncater: truncater,
	}
}

func (c CachingDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	truncatingWriter := internal.WrapWriterWithIntercept(w, func(resp *dns.Msg) {
		c.truncater.TruncateIfNeeded(w, r, resp)
	})

	if !r.RecursionDesired {
		c.next.ServeDNS(truncatingWriter, r)
		return
	}

	_, err := c.ca.ServeDNS(context.TODO(), truncatingWriter, r)
	if err != nil {
		c.logger.Error(c.logTag, "Error getting dns cache:", err.Error())
	}
}

type corednsHandlerWrapper struct {
	Next dns.Handler
}

func (w corednsHandlerWrapper) ServeDNS(ctx context.Context, writer dns.ResponseWriter, m *dns.Msg) (int, error) {
	w.Next.ServeDNS(writer, m)
	return 0, nil
}

func (w corednsHandlerWrapper) Name() string {
	return "CorednsHandlerWrapper"
}

package handlers

import (
	"context"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/miekg/dns"

	"bosh-dns/dns/server/handlers/internal"
	"bosh-dns/dns/server/records/dnsresolver"
)

type CachingDNSHandler struct {
	next      dns.Handler
	ca        *cache.Cache
	logger    boshlog.Logger
	logTag    string
	truncater dnsresolver.ResponseTruncater
	clock     clock.Clock
}

type requestContext struct {
	fromCache bool
}

func NewCachingDNSHandler(next dns.Handler, truncater dnsresolver.ResponseTruncater, clock clock.Clock, logger boshlog.Logger) CachingDNSHandler {
	ca := cache.New()
	ca.Next = corednsHandlerWrapper{Next: next}
	ca.Zones = []string{"."}
	return CachingDNSHandler{
		ca:        ca,
		logTag:    "CachingDNSHandler",
		next:      next,
		truncater: truncater,
		clock:     clock,
		logger:    logger,
	}
}

func (c CachingDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	internal.LogReceivedRequest(c.logger, c, c.logTag, r)
	var dnsMsg *dns.Msg
	truncatingWriter := internal.WrapWriterWithIntercept(w, func(resp *dns.Msg) {
		dnsMsg = resp
		c.truncater.TruncateIfNeeded(w, r, resp)
	})

	if !r.RecursionDesired {
		c.next.ServeDNS(truncatingWriter, r)
		return
	}

	indicator := &requestContext{
		fromCache: true,
	}
	requestContext := context.WithValue(context.Background(), "indicator", indicator)

	before := c.clock.Now()
	_, err := c.ca.ServeDNS(requestContext, truncatingWriter, r)
	duration := c.clock.Now().Sub(before).Nanoseconds()

	if err != nil {
		c.logger.Error(c.logTag, "Error getting dns cache:", err.Error())
	}
	if indicator.fromCache {
		internal.LogRequest(c.logger, c, c.logTag, duration, r, dnsMsg, "")
	}
}

type corednsHandlerWrapper struct {
	Next dns.Handler
}

func (w corednsHandlerWrapper) ServeDNS(ctx context.Context, writer dns.ResponseWriter, m *dns.Msg) (int, error) {
	requestContext := ctx.Value("indicator").(*requestContext)
	requestContext.fromCache = false

	w.Next.ServeDNS(writer, m)
	return 0, nil
}

func (w corednsHandlerWrapper) Name() string {
	return "CorednsHandlerWrapper"
}

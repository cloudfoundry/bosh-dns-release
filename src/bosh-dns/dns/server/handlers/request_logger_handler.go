package handlers

import (
	"code.cloudfoundry.org/clock"
	"fmt"

	"bosh-dns/dns/server/handlers/internal"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type RequestLoggerHandler struct {
	Handler dns.Handler
	clock   clock.Clock
	logger  logger.Logger
	logTag  string
}

func NewRequestLoggerHandler(child dns.Handler, clock clock.Clock, logger logger.Logger) RequestLoggerHandler {
	return RequestLoggerHandler{
		Handler: child,
		clock:   clock,
		logger:  logger,
		logTag:  "RequestLoggerHandler",
	}
}

func (h RequestLoggerHandler) ServeDNS(responseWriter dns.ResponseWriter, req *dns.Msg) {
	var dnsMsg *dns.Msg
	respWriter := internal.WrapWriterWithIntercept(responseWriter, func(msg *dns.Msg) {
		dnsMsg=msg
	})

	before := h.clock.Now()

	h.Handler.ServeDNS(respWriter, req)

	duration := h.clock.Now().Sub(before).Nanoseconds()

	types := make([]string, len(req.Question))
	domains := make([]string, len(req.Question))

	for i, q := range req.Question {
		types[i] = fmt.Sprintf("%d", q.Qtype)
		domains[i] = q.Name
	}

	internal.LogRequest(h.logger, h.Handler, h.logTag, duration, req, dnsMsg.Rcode, "")
}

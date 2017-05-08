package handlers

import (
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal"
	"github.com/miekg/dns"
	"strings"
)

type RequestLoggerHandler struct {
	child      dns.Handler
	clock      clock.Clock
	logger     logger.Logger
	logTag     string
}

func NewRequestLoggerHandler(child dns.Handler, clock clock.Clock, logger logger.Logger) RequestLoggerHandler {
	return RequestLoggerHandler{
		child:  child,
		clock:  clock,
		logger: logger,
		logTag: "RequestLoggerHandler",
	}
}

func (h RequestLoggerHandler) ServeDNS(responseWriter dns.ResponseWriter, req *dns.Msg) {
	var respRcode int
	respWriter := internal.WrapWriterWithIntercept(responseWriter, func(msg *dns.Msg) {
		respRcode = msg.Rcode
	})

	before := h.clock.Now()

	h.child.ServeDNS(respWriter, req)

	duration := h.clock.Now().Sub(before).Nanoseconds()

	types := make([]string, len(req.Question))
	domains := make([]string, len(req.Question))

	for i, q := range req.Question {
		types[i] = fmt.Sprintf("%d", q.Qtype)
		domains[i] = q.Name
	}
	h.logger.Info(h.logTag, fmt.Sprintf("%T Request [%s] [%s] %d %dns",
		h.child,
		strings.Join(types, ","),
		strings.Join(domains, ","),
		respRcode,
		duration,
	))
}

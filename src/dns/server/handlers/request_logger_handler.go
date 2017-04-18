package handlers

import (
	"fmt"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/miekg/dns"
	"net"
	"strings"
)

type RequestLoggerHandler struct {
	muxPattern string
	child      dns.Handler
	clock      clock.Clock
	logger     logger.Logger
	logTag     string
}

func NewRequestLoggerHandler(muxPattern string, child dns.Handler, clock clock.Clock, logger logger.Logger) RequestLoggerHandler {
	return RequestLoggerHandler{
		muxPattern: muxPattern,
		child:      child,
		clock:      clock,
		logger:     logger,
		logTag:     "RequestLoggerHandler",
	}
}

func (h RequestLoggerHandler) ServeDNS(resp dns.ResponseWriter, req *dns.Msg) {
	respWriter := newWrappedRespWriter(resp)

	before := h.clock.Now()

	h.child.ServeDNS(&respWriter, req)

	duration := h.clock.Now().Sub(before).Nanoseconds()

	types := make([]string, len(req.Question))
	for i, q := range req.Question {
		types[i] = fmt.Sprintf("%d", q.Qtype)
	}
	h.logger.Info(h.logTag, fmt.Sprintf("Request [%s] %s %d %dns", strings.Join(types, ","), h.muxPattern, respWriter.respRcode, duration))
}

func newWrappedRespWriter(resp dns.ResponseWriter) respWriterWrapper {
	return respWriterWrapper{
		child: resp,
	}
}

type respWriterWrapper struct {
	respRcode int
	child     dns.ResponseWriter
}

func (r *respWriterWrapper) WriteMsg(m *dns.Msg) error {
	r.respRcode = m.Rcode
	return r.child.WriteMsg(m)
}

func (r *respWriterWrapper) Write(b []byte) (int, error) { panic("not implemented, use WriteMsg") }

func (r *respWriterWrapper) LocalAddr() net.Addr   { return r.child.LocalAddr() }
func (r *respWriterWrapper) RemoteAddr() net.Addr  { return r.child.RemoteAddr() }
func (r *respWriterWrapper) Close() error          { return r.child.Close() }
func (r *respWriterWrapper) TsigStatus() error     { return r.child.TsigStatus() }
func (r *respWriterWrapper) TsigTimersOnly(b bool) { r.child.TsigTimersOnly(b) }
func (r *respWriterWrapper) Hijack()               { r.child.Hijack() }

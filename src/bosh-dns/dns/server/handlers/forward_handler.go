package handlers

import (
	"bosh-dns/dns/server/handlers/internal"
	"bosh-dns/dns/server/records/dnsresolver"
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/clock"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type ForwardHandler struct {
	clock            clock.Clock
	recursors        RecursorPool
	exchangerFactory ExchangerFactory
	logger           logger.Logger
	logTag           string
	truncater        dnsresolver.ResponseTruncater
}

//go:generate counterfeiter . Exchanger

type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

type Cache interface {
	Get(req *dns.Msg) *dns.Msg
	Write(req, answer *dns.Msg)
	GetExpired(*dns.Msg) *dns.Msg
}

func NewForwardHandler(recursors RecursorPool, exchangerFactory ExchangerFactory, clock clock.Clock, logger logger.Logger, truncater dnsresolver.ResponseTruncater) ForwardHandler {
	return ForwardHandler{
		recursors:        recursors,
		exchangerFactory: exchangerFactory,
		clock:            clock,
		logger:           logger,
		logTag:           "ForwardHandler",
		truncater:        truncater,
	}
}

func (r ForwardHandler) ServeDNS(responseWriter dns.ResponseWriter, request *dns.Msg) {
	before := r.clock.Now()

	if len(request.Question) == 0 {
		r.writeEmptyMessage(responseWriter, request)
		return
	}

	network := r.network(responseWriter)

	client := r.exchangerFactory(network)

	err := r.recursors.PerformStrategically(func(recursor string) error {
		exchangeAnswer, _, err := client.Exchange(request, recursor)

		if exchangeAnswer != nil && exchangeAnswer.MsgHdr.Rcode == dns.RcodeServerFailure {
			err = fmt.Errorf("Received SERVFAIL from upstream (recursor: %s)", recursor)
		}

		if err == nil {
			r.truncater.TruncateIfNeeded(responseWriter, request, exchangeAnswer)

			if writeErr := responseWriter.WriteMsg(exchangeAnswer); writeErr != nil {
				r.logger.Error(r.logTag, "error writing response: %s", writeErr.Error())
			} else {
				r.logRecursor(before, request, exchangeAnswer.Rcode, "recursor="+recursor)
			}

			return nil
		}

		r.logger.Debug(r.logTag, "error recursing to %q: %s", recursor, err.Error())
		return err
	})

	if err != nil {
		r.writeNoResponseMessage(responseWriter, request)
		r.logRecursor(before, request, dns.RcodeServerFailure, "error=["+err.Error() + "]")
	}
}

func (r ForwardHandler) logRecursor(before time.Time, request *dns.Msg, code int, recursor string) {
	duration := r.clock.Now().Sub(before).Nanoseconds()
	internal.LogRequest(r.logger, r, r.logTag, duration, request, code, recursor)
}

func (ForwardHandler) network(responseWriter dns.ResponseWriter) string {
	network := "udp"
	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	return network
}

func (r ForwardHandler) writeNoResponseMessage(responseWriter dns.ResponseWriter, req *dns.Msg) {
	responseMessage := &dns.Msg{}
	responseMessage.SetReply(req)
	responseMessage.SetRcode(req, dns.RcodeServerFailure)
	if err := responseWriter.WriteMsg(responseMessage); err != nil {
		r.logger.Error(r.logTag, "error writing response: %s", err.Error())
	}
}

func (r ForwardHandler) writeEmptyMessage(responseWriter dns.ResponseWriter, req *dns.Msg) {
	emptyMessage := &dns.Msg{}
	r.logger.Debug(r.logTag, "received a request with no questions")
	emptyMessage.Authoritative = true
	emptyMessage.SetRcode(req, dns.RcodeSuccess)
	if err := responseWriter.WriteMsg(emptyMessage); err != nil {
		r.logger.Error(r.logTag, "error writing response: %s", err.Error())
	}
}

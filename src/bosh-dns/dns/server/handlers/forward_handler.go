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
	internal.LogReceivedRequest(r.logger, r, r.logTag, request)
	before := r.clock.Now()

	if len(request.Question) == 0 {
		r.writeEmptyMessage(responseWriter, request)
		return
	}

	network := r.network(responseWriter)

	client := r.exchangerFactory(network)

	err := r.recursors.PerformStrategically(func(recursor string) error {
		exchangeAnswer, _, err := client.Exchange(request, recursor)

		if err != nil {
			question := request.Question[0].Name
			r.logger.Error(r.logTag, "error recursing for %s to %q: %s", question, recursor, err.Error())
		}

		if exchangeAnswer != nil && exchangeAnswer.MsgHdr.Rcode != dns.RcodeSuccess {
			question := request.Question[0].Name
			err = fmt.Errorf("received %s for %s from upstream (recursor: %s)", dns.RcodeToString[exchangeAnswer.MsgHdr.Rcode], question, recursor)
			if exchangeAnswer.MsgHdr.Rcode == dns.RcodeNameError {
				r.logger.Debug(r.logTag, "error recursing to %q: %s", recursor, err.Error())
			} else {
				r.logger.Error(r.logTag, "error recursing to %q: %s", recursor, err.Error())
			}
		}

		if err != nil {
			return err
		}

		r.truncater.TruncateIfNeeded(responseWriter, request, exchangeAnswer)

		r.logRecursor(before, request, exchangeAnswer, "recursor="+recursor)
		if writeErr := responseWriter.WriteMsg(exchangeAnswer); writeErr != nil {
			r.logger.Error(r.logTag, "error writing response: %s", writeErr.Error())
		}

		return nil
	})

	if err != nil {
		r.writeNoResponseMessage(responseWriter, request, before, "error=["+err.Error()+"]", err)
	}
}

func (r ForwardHandler) logRecursor(before time.Time, request *dns.Msg, response *dns.Msg, recursor string) {
	duration := r.clock.Now().Sub(before).Nanoseconds()
	internal.LogRequest(r.logger, r, r.logTag, duration, request, response, recursor)
}

func (ForwardHandler) network(responseWriter dns.ResponseWriter) string {
	network := "udp"
	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	return network
}

func (r ForwardHandler) writeNoResponseMessage(responseWriter dns.ResponseWriter, req *dns.Msg, before time.Time, recursor string, err error) {
	responseMessage := &dns.Msg{}
	responseMessage.SetReply(req)

	switch err.(type) {
	case net.Error:
		if err.(net.Error).Timeout() {
			responseMessage.SetRcode(req, dns.RcodeServerFailure)
			break
		}
	default:
		responseMessage.SetRcode(req, dns.RcodeNameError)
		break
	}

	r.logRecursor(before, req, responseMessage, recursor)
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

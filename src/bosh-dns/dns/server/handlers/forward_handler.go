package handlers

import (
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"

	"bosh-dns/dns/server"
	"bosh-dns/dns/server/handlers/internal"
	"bosh-dns/dns/server/records/dnsresolver"
)

type ForwardHandler struct {
	clock            clock.Clock
	recursors        RecursorPool
	exchangerFactory ExchangerFactory
	logger           logger.Logger
	logTag           string
	truncater        dnsresolver.ResponseTruncater
}

//counterfeiter:generate . Exchanger

type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

type Cache interface {
	Get(req *dns.Msg) *dns.Msg
	Write(req, answer *dns.Msg)
	GetExpired(*dns.Msg) *dns.Msg
}

func NewForwardHandler(
	recursors RecursorPool,
	exchangerFactory ExchangerFactory,
	clock clock.Clock,
	logger logger.Logger,
	truncater dnsresolver.ResponseTruncater,
) ForwardHandler {
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
	fmt.Printf("Hello! I am in ForwardHandler::ServeDNS. My request is '%s'\n", request.String())
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
			fmt.Printf("After calling Exchange. The error is '%s'\n", err.Error())
		}
		if err == nil {
			fmt.Printf("After calling Exchange. There is no error.'\n")
		}

		if err != nil {
			question := request.Question[0].Name
			r.logger.Error(r.logTag, "error recursing for %s to %q: %s", question, recursor, err.Error())
		}
		if exchangeAnswer != nil && exchangeAnswer.MsgHdr.Rcode != dns.RcodeSuccess {
			question := request.Question[0].Name
			err = server.NewDnsError(exchangeAnswer.MsgHdr.Rcode, question, recursor)
			if exchangeAnswer.MsgHdr.Rcode == dns.RcodeNameError {
				r.logger.Debug(r.logTag, "error recursing to %q: %s", recursor, err.Error())
			} else {
				r.logger.Error(r.logTag, "error recursing to %q: %s", recursor, err.Error())
			}
		}

		if err != nil {
			fmt.Printf("After the error checking. The error is '%s'\n", err.Error())
		}
		if err == nil {
			fmt.Printf("After the error checking. There is no error!\n")
		}

		if err != nil {
			return err
		}

		fmt.Printf("Before TruncateIfNeeded. ExchangeAnswer: '%s'\n", exchangeAnswer)
		r.truncater.TruncateIfNeeded(responseWriter, request, exchangeAnswer)
		fmt.Printf("After TruncateIfNeeded. ExchangeAnswer: '%s'\n", exchangeAnswer)

		r.logRecursor(before, request, exchangeAnswer, "recursor="+recursor)
		if writeErr := responseWriter.WriteMsg(exchangeAnswer); writeErr != nil {
			r.logger.Error(r.logTag, "error writing response: %s", writeErr.Error())
		}

		fmt.Printf("After responseWriter.WriteMsg. ExchangeAnswer: '%s'\n", exchangeAnswer)

		return nil
	})

	if err != nil {
		responseMessage := r.createResponseFromError(request, err)
		r.logRecursor(before, request, responseMessage, "error=["+err.Error()+"]")
		if err := responseWriter.WriteMsg(responseMessage); err != nil {
			r.logger.Error(r.logTag, "error writing response: %s", err.Error())
		}
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

func (r ForwardHandler) createResponseFromError(req *dns.Msg, err error) *dns.Msg {
	responseMessage := &dns.Msg{}
	responseMessage.SetReply(req)

	switch err := err.(type) {
	case net.Error:
		responseMessage.SetRcode(req, dns.RcodeServerFailure)
	case server.DnsError:
		if err.Rcode() == dns.RcodeServerFailure {
			responseMessage.SetRcode(req, dns.RcodeServerFailure)
		} else {
			responseMessage.SetRcode(req, dns.RcodeNameError)
		}
		break //nolint:gosimple
	default:
		responseMessage.SetRcode(req, dns.RcodeNameError)
		break //nolint:gosimple
	}

	return responseMessage
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

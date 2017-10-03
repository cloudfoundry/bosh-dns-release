package handlers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"

	"bosh-dns/dns/server/handlers/internal"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type ForwardHandler struct {
	clock            clock.Clock
	recursors        internal.RecursorPool
	exchangerFactory ExchangerFactory
	logger           logger.Logger
	logTag           string
}

//go:generate counterfeiter . Exchanger
type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

//go:generate counterfeiter . Cache
type Cache interface {
	Get(req *dns.Msg) *dns.Msg
	Write(req, answer *dns.Msg)
	GetExpired(*dns.Msg) *dns.Msg
}

func NewForwardHandler(recursors []string, exchangerFactory ExchangerFactory, clock clock.Clock, logger logger.Logger) ForwardHandler {
	return ForwardHandler{
		recursors:        internal.NewFailoverRecursorPool(recursors),
		exchangerFactory: exchangerFactory,
		clock:            clock,
		logger:           logger,
		logTag:           "ForwardHandler",
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
		if err == nil || err == dns.ErrTruncated {
			response := r.compressIfNeeded(responseWriter, request, exchangeAnswer)

			if writeErr := responseWriter.WriteMsg(response); writeErr != nil {
				r.logger.Error(r.logTag, "error writing response: %s", writeErr.Error())
			} else {
				r.logRecursor(before, request, response.Rcode, "recursor="+recursor)
			}

			return nil
		}

		r.logger.Debug(r.logTag, "error recursing to %q: %s", recursor, err.Error())
		return err
	})

	if err != nil {
		recursionAvailable := true
		if _, ok := err.(internal.NoRecursorsError); ok {
			recursionAvailable = false
		}

		r.writeNoResponseMessage(responseWriter, request, recursionAvailable)
		r.logRecursor(before, request, dns.RcodeServerFailure, err.Error())
	}
}

func (r ForwardHandler) logRecursor(before time.Time, request *dns.Msg, code int, recursor string) {
	duration := r.clock.Now().Sub(before).Nanoseconds()
	types := make([]string, len(request.Question))
	domains := make([]string, len(request.Question))

	for i, q := range request.Question {
		types[i] = fmt.Sprintf("%d", q.Qtype)
		domains[i] = q.Name
	}
	r.logger.Info(r.logTag, fmt.Sprintf("%T Request [%s] [%s] %d [%s] %dns",
		r,
		strings.Join(types, ","),
		strings.Join(domains, ","),
		code,
		recursor,
		duration,
	))
}

func (r ForwardHandler) compressIfNeeded(responseWriter dns.ResponseWriter, request, response *dns.Msg) *dns.Msg {
	if _, ok := responseWriter.RemoteAddr().(*net.UDPAddr); ok {
		maxUDPSize := 512
		if opt := request.IsEdns0(); opt != nil {
			maxUDPSize = int(opt.UDPSize())
		}

		if response.Len() > maxUDPSize {
			r.logger.Debug(r.logTag, "Setting compress flag on msg id:", request.Id)

			responseCopy := dns.Msg(*response)
			responseCopy.Compress = true

			return &responseCopy
		}
	}

	return response
}

func (ForwardHandler) network(responseWriter dns.ResponseWriter) string {
	network := "udp"
	if _, ok := responseWriter.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	return network
}

func (r ForwardHandler) writeNoResponseMessage(responseWriter dns.ResponseWriter, req *dns.Msg, recursionAvailable bool) {
	responseMessage := &dns.Msg{}
	responseMessage.SetReply(req)
	responseMessage.RecursionAvailable = recursionAvailable
	responseMessage.Authoritative = false
	responseMessage.SetRcode(req, dns.RcodeServerFailure)
	if err := responseWriter.WriteMsg(responseMessage); err != nil {
		r.logger.Error(r.logTag, "error writing response: %s", err.Error())
	}
}

func (r ForwardHandler) writeEmptyMessage(responseWriter dns.ResponseWriter, req *dns.Msg) {
	emptyMessage := &dns.Msg{}
	r.logger.Info(r.logTag, "received a request with no questions")
	emptyMessage.RecursionAvailable = false
	emptyMessage.Authoritative = true
	emptyMessage.SetRcode(req, dns.RcodeSuccess)
	if err := responseWriter.WriteMsg(emptyMessage); err != nil {
		r.logger.Error(r.logTag, "error writing response: %s", err.Error())
	}
}

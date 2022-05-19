package monitoring

import (
	"context"
	"errors"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DNSRequestType defines which type of dns request is handled
type DNSRequestType string
type key int

const (
	// DNSRequestTypeInternal is used for internal dns requests
	DNSRequestTypeInternal DNSRequestType = "internal"
	// DNSRequestTypeExternal is used for external dns requests
	DNSRequestTypeExternal DNSRequestType = "external"

	dnsRequestContext key = iota
)

// NewRequestContext Creates a new context for the given request typeß
func NewRequestContext(t DNSRequestType) context.Context {
	return context.WithValue(context.Background(), dnsRequestContext, t)
}

// NewPluginHandlerAdapter creates a new PluginHandler for both internal and external dns requests
func NewPluginHandlerAdapter(internalHandler dns.Handler, externalHandler dns.Handler, requestManager RequestCounter) pluginHandlerAdapter {
	return pluginHandlerAdapter{internalHandler: internalHandler, externalHandler: externalHandler, requestManager: requestManager}
}

type pluginHandlerAdapter struct {
	internalHandler dns.Handler
	externalHandler dns.Handler
	requestManager  RequestCounter
}

func (p pluginHandlerAdapter) Name() string {
	return "pluginHandlerAdapter"
}

//go:generate counterfeiter . RequestCounter

type RequestCounter interface {
	IncrementExternalCounter()
	IncrementInternalCounter()
}

type RequestManager struct {
	externalRequestsCounter prometheus.Counter
	internalRequestsCounter prometheus.Counter
}

func NewRequestManager() RequestManager {
	extReqs := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "boshdns",
		Subsystem: "requests",
		Name:      "external_total",
		Help:      "The count of external requests.",
	})
	intReqs := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "boshdns",
		Subsystem: "requests",
		Name:      "internal_total",
		Help:      "The count of internal requests.",
	})
	return RequestManager{externalRequestsCounter: extReqs, internalRequestsCounter: intReqs}
}

func (m RequestManager) IncrementExternalCounter() {
	m.externalRequestsCounter.Inc()
}

func (m RequestManager) IncrementInternalCounter() {
	m.internalRequestsCounter.Inc()
}

func (p pluginHandlerAdapter) ServeDNS(ctx context.Context, writer dns.ResponseWriter, m *dns.Msg) (int, error) {
	v := ctx.Value(dnsRequestContext)

	if v == nil {
		return 0, errors.New("No DNS request type found in context")
	}

	if p.externalHandler != nil && v == DNSRequestTypeExternal {
		p.externalHandler.ServeDNS(writer, m)
		p.requestManager.IncrementExternalCounter()
	} else if p.internalHandler != nil && v == DNSRequestTypeInternal {
		p.internalHandler.ServeDNS(writer, m)
		p.requestManager.IncrementInternalCounter()
	}
	return 0, nil
}

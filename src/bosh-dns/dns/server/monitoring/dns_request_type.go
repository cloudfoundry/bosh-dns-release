package monitoring

import (
	"context"
	"errors"
	"github.com/miekg/dns"
)

// DNSRequestType defines which type of dns request is handled
type DNSRequestType string
type key int

const(
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
func NewPluginHandlerAdapter(internalHandler dns.Handler, externalHandler dns.Handler) pluginHandlerAdapter {
	return pluginHandlerAdapter{internalHandler: internalHandler, externalHandler: externalHandler}
}

type pluginHandlerAdapter struct {
	internalHandler dns.Handler
	externalHandler dns.Handler
}

func(p pluginHandlerAdapter) Name() string {
	return "pluginHandlerAdapter"
}

func(p pluginHandlerAdapter) ServeDNS(ctx context.Context, writer dns.ResponseWriter, m *dns.Msg) (int, error) {
	v := ctx.Value(dnsRequestContext)

	if v == nil {
		return 0, errors.New("No DNS request type found in context")
	}

	if p.externalHandler != nil && v == DNSRequestTypeExternal {
		p.externalHandler.ServeDNS(writer, m)
	} else if p.internalHandler != nil && v == DNSRequestTypeInternal {
		p.internalHandler.ServeDNS(writer, m)
	}
	return 0, nil
}


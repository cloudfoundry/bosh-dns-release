package handlers

import (
	"bosh-dns/dns/server/monitoring"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type MetricsDNSHandler struct {
	metricsReporter monitoring.MetricsReporter
	next            dns.Handler
}

func NewMetricsDNSHandler(metricsReporter monitoring.MetricsReporter, next dns.Handler) MetricsDNSHandler {
	return MetricsDNSHandler{
		metricsReporter: metricsReporter,
		next:            next,
	}
}

func (m MetricsDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m.metricsReporter.Report(context.Background(), w, r)
	m.next.ServeDNS(w, r)
}

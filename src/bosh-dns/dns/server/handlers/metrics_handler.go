package handlers

import (
	"github.com/miekg/dns"

	"bosh-dns/dns/server/monitoring"
)

type MetricsDNSHandler struct {
	metricsReporter monitoring.MetricsReporter
	requestType     monitoring.DNSRequestType
}

func NewMetricsDNSHandler(metricsReporter monitoring.MetricsReporter, requestType monitoring.DNSRequestType) MetricsDNSHandler {
	return MetricsDNSHandler{
		metricsReporter: metricsReporter,
		requestType:     requestType,
	}
}

func (m MetricsDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	requestContext := monitoring.NewRequestContext(m.requestType)
	m.metricsReporter.Report(requestContext, w, r) //nolint:errcheck
}

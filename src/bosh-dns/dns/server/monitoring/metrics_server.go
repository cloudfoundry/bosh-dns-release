package monitoring

import (
	"context"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/miekg/dns"
)

//go:generate counterfeiter . MetricsReporter
//go:generate counterfeiter . CoreDNSMetricsServer

type MetricsReporter interface {
	Report(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)
}

type CoreDNSMetricsServer interface {
	OnStartup() error
	OnFinalShutdown() error
	ServeDNS(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)
}

type MetricsServerWrapper struct {
	coreDNSServer CoreDNSMetricsServer
	logger        boshlog.Logger
	logTag        string
}

func NewMetricsServerWrapper(logger boshlog.Logger, server CoreDNSMetricsServer) *MetricsServerWrapper {
	return &MetricsServerWrapper{
		coreDNSServer: server,
		logTag:        "MetricsServer",
		logger:        logger,
	}
}

func MetricsServer(listenAddress string) CoreDNSMetricsServer {
	return metrics.New(listenAddress)
}

func (m *MetricsServerWrapper) MetricsReporter() MetricsReporter {
	return m
}

func (m *MetricsServerWrapper) Report(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return m.coreDNSServer.ServeDNS(ctx, w, r)
}

func (m *MetricsServerWrapper) Run(shutdown chan struct{}) error {
	if err := m.coreDNSServer.OnStartup(); err != nil {
		return bosherr.WrapError(err, "setting up the metrics listener")
	}
	for {
		select {
		case <-shutdown:
			err := m.coreDNSServer.OnFinalShutdown()
			if err != nil {
				return bosherr.WrapError(err, "tearing down the metrics listener")
			}
			return nil
		}
	}
}

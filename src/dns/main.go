package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"

	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/cloudfoundry/dns-release/src/dns/config"
	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func parseFlags() (string, error) {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath == "" {
		return "", errors.New("--config is a required flag")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", err
	}

	return configPath, nil
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	logger := boshlog.NewAsyncWriterLogger(logger.LevelDebug, os.Stdout, os.Stderr)
	logTag := "main"
	defer logger.FlushTimeout(5 * time.Second)

	configPath, err := parseFlags()
	if err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	c, err := config.LoadFromFile(configPath)
	if err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	mux := dns.NewServeMux()

	addHandler(mux, "bosh.", handlers.NewDiscoveryHandler(logger, records.NewRepo(c.RecordsFile, system.NewOsFileSystem(logger), logger)), logger)
	addHandler(mux, "arpa.", handlers.NewArpaHandler(logger), logger)
	addHandler(mux, "healthcheck.bosh-dns.", handlers.NewHealthCheckHandler(logger), logger)
	addHandler(mux, ".", handlers.NewForwardHandler(c.Recursors, handlers.NewExchangerFactory(time.Duration(c.RecursorTimeout)), logger), logger)

	bindAddress := fmt.Sprintf("%s:%d", c.Address, c.Port)
	shutdown := make(chan struct{})
	dnsServer := server.New(
		[]server.DNSServer{
			&dns.Server{Addr: bindAddress, Net: "tcp", Handler: mux},
			&dns.Server{Addr: bindAddress, Net: "udp", UDPSize: 65535, Handler: mux},
		},
		[]server.HealthCheck{
			server.NewUDPHealthCheck(net.Dial, bindAddress),
			server.NewTCPHealthCheck(net.Dial, bindAddress),
		},
		time.Duration(c.Timeout),
		shutdown,
	)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	go func() {
		<-sigterm
		close(shutdown)
	}()

	if err := dnsServer.Run(); err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	return 0
}

func addHandler(mux *dns.ServeMux, pattern string, handler dns.Handler, logger logger.Logger) {
	mux.Handle(pattern, handlers.NewRequestLoggerHandler(handler, clock.Real, logger))
}

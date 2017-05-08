package main

import (
	"errors"
	"flag"
	"fmt"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/dns-release/src/dns/clock"
	"github.com/cloudfoundry/dns-release/src/dns/config"
	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
	"github.com/cloudfoundry/dns-release/src/dns/shuffle"
	"github.com/miekg/dns"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func parseFlags() (string, error) {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath == "" {
		return "", errors.New("--config is a required flag")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", bosherr.WrapError(err, fmt.Sprintf("Unable to find config file at '%s'", configPath))
	}

	return configPath, nil
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, os.Stdout, os.Stderr)
	logTag := "main"
	defer logger.FlushTimeout(5 * time.Second)

	configPath, err := parseFlags()
	if err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	config, err := config.LoadFromFile(configPath)
	if err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	fs := boshsys.NewOsFileSystem(logger)
	aliasConfiguration, err := aliases.ConfigFromGlob(
		fs,
		aliases.NewFSLoader(fs),
		config.AliasFilesGlob,
	)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("loading alias configuration: %s", err.Error()))
		return 1
	}

	mux := dns.NewServeMux()

	recordsRepo := records.NewRepo(config.RecordsFile, system.NewOsFileSystem(logger), logger)
	localDomain := dnsresolver.NewLocalDomain(logger, recordsRepo, shuffle.New())
	discoveryHandler := handlers.NewDiscoveryHandler(logger, localDomain)
	addHandler(mux, "bosh.", discoveryHandler, logger)
	addHandler(mux, "arpa.", handlers.NewArpaHandler(logger), logger)
	addHandler(mux, "healthcheck.bosh-dns.", handlers.NewHealthCheckHandler(logger), logger)

	forwardHandler := handlers.NewForwardHandler(config.Recursors, handlers.NewExchangerFactory(time.Duration(config.RecursorTimeout)), logger)
	addHandler(mux, ".", forwardHandler, logger)

	aliasResolver, err := handlers.NewAliasResolvingHandler(mux, aliasConfiguration, localDomain, clock.Real, logger)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("could not initiate alias resolving handler: %s", err.Error()))
		return 1
	}

	bindAddress := fmt.Sprintf("%s:%d", config.Address, config.Port)
	shutdown := make(chan struct{})
	dnsServer := server.New(
		[]server.DNSServer{
			&dns.Server{Addr: bindAddress, Net: "tcp", Handler: aliasResolver},
			&dns.Server{Addr: bindAddress, Net: "udp", Handler: aliasResolver, UDPSize: 65535},
		},
		[]server.HealthCheck{
			server.NewUDPHealthCheck(net.Dial, bindAddress),
			server.NewTCPHealthCheck(net.Dial, bindAddress),
		},
		time.Duration(config.Timeout),
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

func addHandler(mux *dns.ServeMux, pattern string, handler dns.Handler, logger boshlog.Logger) {
	mux.Handle(pattern, handlers.NewRequestLoggerHandler(handler, clock.Real, logger))
}

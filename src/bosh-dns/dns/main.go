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

	"code.cloudfoundry.org/clock"

	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"

	dnsconfig "bosh-dns/dns/config"
	"bosh-dns/dns/server"
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/shuffle"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/miekg/dns"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
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

	config, err := dnsconfig.LoadFromFile(configPath)
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
	clock := clock.NewClock()
	repoUpdate := make(chan struct{})

	dnsManager := newDNSManager(logger, clock, fs)
	recursorReader := dnsconfig.NewRecursorReader(dnsManager, config.Address)
	err = dnsconfig.ConfigureRecursors(recursorReader, &config)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to configure recursor addresses from os: %s", err.Error()))
		return 1
	}

	var healthWatcher healthiness.HealthWatcher = healthiness.NewNopHealthWatcher()
	if config.Health.Enabled {
		httpClient, err := setupSecureGet(config.Health.CAFile, config.Health.CertificateFile, config.Health.PrivateKeyFile)
		if err != nil {
			logger.Error(logTag, fmt.Sprintf("Unable to configure health checker %s", err.Error()))
			return 1
		}
		healthChecker := healthiness.NewHealthChecker(httpclient.NewHTTPClient(httpClient, logger), config.Health.Port)
		checkInterval := time.Duration(config.Health.CheckInterval)
		healthWatcher = healthiness.NewHealthWatcher(healthChecker, clock, checkInterval)
	}

	recordsRepo := records.NewRepo(config.RecordsFile, system.NewOsFileSystem(logger), clock, logger, repoUpdate)
	localDomain := dnsresolver.NewLocalDomain(logger, recordsRepo, shuffle.New(), healthWatcher)
	discoveryHandler := handlers.NewDiscoveryHandler(logger, localDomain)

	handlerRegistrar := handlers.NewHandlerRegistrar(logger, clock, recordsRepo, mux, discoveryHandler)

	handlers.AddHandler(mux, clock, "arpa.", handlers.NewArpaHandler(logger), logger)

	healthchecks := []server.HealthCheck{}
	for _, healthCheckDomain := range config.HealthcheckDomains {
		handlers.AddHandler(mux, clock, healthCheckDomain, handlers.NewHealthCheckHandler(logger), logger)
		healthchecks = append(healthchecks, server.NewAnswerValidatingHealthCheck(fmt.Sprintf("%s:%d", config.Address, config.Port), healthCheckDomain, "udp"))
		healthchecks = append(healthchecks, server.NewAnswerValidatingHealthCheck(fmt.Sprintf("%s:%d", config.Address, config.Port), healthCheckDomain, "tcp"))
	}

	forwardHandler := handlers.NewForwardHandler(config.Recursors, handlers.NewExchangerFactory(time.Duration(config.RecursorTimeout)), logger)
	handlers.AddHandler(mux, clock, ".", forwardHandler, logger)

	aliasResolver, err := handlers.NewAliasResolvingHandler(mux, aliasConfiguration, localDomain, clock, logger)
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
		healthchecks,
		time.Duration(config.Timeout),
		time.Duration(5*time.Second),
		shutdown,
		logger,
	)

	go func() {
		err := handlerRegistrar.Run(shutdown)
		if err != nil {
			logger.Error(logTag, fmt.Sprintf("could not start handler registrar: %s", err.Error()))
		}
	}()

	go healthWatcher.Run(shutdown)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	go func() {
		<-sigterm
		close(repoUpdate)
		close(shutdown)
	}()

	if err := dnsServer.Run(); err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	return 0
}

func setupSecureGet(caFile, clientCertFile, clientKeyFile string) (*http.Client, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithIdentity(cert),
		tlsconfig.WithPivotalDefaults(),
	)

	clientConfig := tlsConfig.Client(tlsconfig.WithAuthority(caCertPool))
	clientConfig.BuildNameToCertificate()
	clientConfig.ServerName = "health.bosh-dns"

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{TLSClientConfig: clientConfig, Dial: dialer.Dial}
	return &http.Client{Transport: transport}, nil
}

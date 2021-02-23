package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"code.cloudfoundry.org/clock"

	"bosh-dns/dns/api"
	dnsconfig "bosh-dns/dns/config"
	addressesconfig "bosh-dns/dns/config/addresses"
	handlersconfig "bosh-dns/dns/config/handlers"
	"bosh-dns/dns/server"
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/monitoring"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/shuffle"
	"bosh-dns/healthconfig"
	"bosh-dns/tlsclient"

	"code.cloudfoundry.org/tlsconfig"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/miekg/dns"
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
	configPath, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	config, err := dnsconfig.LoadFromFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	level, err := config.GetLogLevel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	logger := boshlog.NewAsyncWriterLogger(level, os.Stdout)
	if config.UseRFC3339Formatting() {
		logger.UseRFC3339Timestamps()
	}
	logTag := "main"
	logger.Info(logTag, "bosh-dns starting")
	defer logger.FlushTimeout(5 * time.Second)

	fs := boshsys.NewOsFileSystem(logger)

	addressConfiguration, err := addressesconfig.ConfigFromGlob(
		fs,
		addressesconfig.NewFSLoader(fs),
		config.AddressesFilesGlob,
	)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("loading addresses configuration: %s", err.Error()))
		return 1
	}

	aliasConfiguration, err := aliases.ConfigFromGlob(
		fs,
		aliases.NewFSLoader(fs),
		config.AliasFilesGlob,
	)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("loading alias configuration: %s", err.Error()))
		return 1
	}

	handlersConfiguration, err := handlersconfig.ConfigFromGlob(
		fs,
		handlersconfig.NewFSLoader(fs),
		config.HandlersFilesGlob,
	)

	if err != nil {
		logger.Error(logTag, fmt.Sprintf("loading handlers configuration: %s", err.Error()))
		return 1
	}

	listenIPs := []string{config.Address}
	for _, addr := range addressConfiguration {
		listenIPs = append(listenIPs, addr.Address)
	}

	mux := dns.NewServeMux()
	clock := clock.NewClock()
	repoUpdate := make(chan struct{})

	dnsManager := newDNSManager(config.Address, logger, clock, fs)
	recursorReader := dnsconfig.NewRecursorReader(dnsManager, listenIPs)
	stringShuffler := shuffle.NewStringShuffler()
	err = dnsconfig.ConfigureRecursors(recursorReader, stringShuffler, &config)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("Unable to configure recursor addresses from os: %s", err.Error()))
		return 1
	}
	logger.Debug(logTag, fmt.Sprintf("Upstream recursors are configured to %v with excluded recursors %v", config.Recursors, config.ExcludedRecursors))

	var healthWatcher healthiness.HealthWatcher = healthiness.NewNopHealthWatcher()
	var healthChecker healthiness.HealthChecker = healthiness.NewDisabledHealthChecker()
	if config.Health.Enabled {
		httpClient, err := tlsclient.NewFromFiles("health.bosh-dns", config.Health.CAFile, config.Health.CertificateFile, config.Health.PrivateKeyFile, time.Duration(config.RequestTimeout), logger)
		if err != nil {
			logger.Error(logTag, fmt.Sprintf("Unable to configure health checker %s", err.Error()))
			return 1
		}
		healthChecker = healthiness.NewHealthChecker(httpClient, config.Health.Port, logger)
		checkInterval := time.Duration(config.Health.CheckInterval)
		healthWatcher = healthiness.NewHealthWatcher(1000, healthChecker, clock, checkInterval, logger)
	}

	shutdown := make(chan struct{})

	fileReader := records.NewFileReader(config.RecordsFile, system.NewOsFileSystem(logger), clock, logger, repoUpdate)
	filtererFactory := records.NewHealthFiltererFactory(healthWatcher, time.Duration(config.Health.SynchronousCheckTimeout))
	recordSet, err := records.NewRecordSet(fileReader, aliasConfiguration, healthWatcher, uint(config.Health.MaxTrackedQueries), shutdown, logger, filtererFactory, records.NewAliasEncoder())

	truncater := dnsresolver.NewResponseTruncater()
	localDomain := dnsresolver.NewLocalDomain(logger, recordSet, shuffle.New(), truncater)

	recursorPool := handlers.NewFailoverRecursorPool(config.Recursors, config.RecursorSelection, config.RecursorRetryCount, logger)
	exchangerFactory := handlers.NewExchangerFactory(time.Duration(config.RecursorTimeout))
	forwardHandler := handlers.NewForwardHandler(recursorPool, exchangerFactory, clock, logger, truncater)

	mux.Handle("arpa.", handlers.NewRequestLoggerHandler(handlers.NewArpaHandler(logger, recordSet, forwardHandler), clock, logger))

	handlerFactory := handlers.NewFactory(exchangerFactory, clock, stringShuffler, config.RecursorRetryCount, logger, truncater)

	delegatingHandlers, err := handlersConfiguration.GenerateHandlers(handlerFactory)
	if err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}
	for domain, handler := range delegatingHandlers {
		mux.Handle(domain, handlers.NewRequestLoggerHandler(handler, clock, logger))
	}

	listenAddrs := []string{fmt.Sprintf("%s:%d", config.Address, config.Port)}
	for _, addr := range addressConfiguration {
		listenAddrs = append(listenAddrs, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
	}

	upchecks := []server.Upcheck{}
	for _, upcheckDomain := range config.UpcheckDomains {
		mux.Handle(upcheckDomain, handlers.NewRequestLoggerHandler(handlers.NewUpcheckHandler(logger), clock, logger))
		for _, addr := range listenAddrs {
			upchecks = append(upchecks, server.NewDNSAnswerValidatingUpcheck(addr, upcheckDomain, "udp", logger))
			upchecks = append(upchecks, server.NewDNSAnswerValidatingUpcheck(addr, upcheckDomain, "tcp", logger))
			if config.InternalUpcheckDomain.Enabled {
				upchecks = append(upchecks, server.NewInternalDNSAnswerValidatingUpcheck(addr, config.InternalUpcheckDomain.DNSQuery, "udp", logger))
				upchecks = append(upchecks, server.NewInternalDNSAnswerValidatingUpcheck(addr, config.InternalUpcheckDomain.DNSQuery, "tcp", logger))
			}
		}
	}

	var (
		nextInternalHandler  dns.Handler = handlers.NewDiscoveryHandler(logger, localDomain)
		nextExternalHandler  dns.Handler = forwardHandler
		metricsServerWrapper *monitoring.MetricsServerWrapper
	)
	if config.Cache.Enabled {
		nextExternalHandler = handlers.NewCachingDNSHandler(nextExternalHandler, truncater, clock, logger)
	}
	if config.Metrics.Enabled {
		metricsAddr := fmt.Sprintf("%s:%d", config.Metrics.Address, config.Metrics.Port)
		metricsServerWrapper = monitoring.NewMetricsServerWrapper(logger, monitoring.MetricsServer(metricsAddr, nextInternalHandler, nextExternalHandler))
		nextExternalHandler = handlers.NewMetricsDNSHandler(metricsServerWrapper.MetricsReporter(), monitoring.DNSRequestTypeExternal)
		nextInternalHandler = handlers.NewMetricsDNSHandler(metricsServerWrapper.MetricsReporter(), monitoring.DNSRequestTypeInternal)
	}
	mux.Handle(".", nextExternalHandler)

	servers := []server.DNSServer{}
	numListeners := runtime.NumCPU()
	if runtime.GOOS == "windows" {
		numListeners = 1
	}

	for _, addr := range listenAddrs {
		for i := 0; i < numListeners; i++ {
			servers = append(servers,
				&dns.Server{Addr: addr, Net: "tcp", Handler: mux, ReadTimeout: time.Duration(config.RequestTimeout), WriteTimeout: time.Duration(config.RequestTimeout), ReusePort: true},
				&dns.Server{Addr: addr, Net: "udp", Handler: mux, ReadTimeout: time.Duration(config.RequestTimeout), WriteTimeout: time.Duration(config.RequestTimeout), ReusePort: true, UDPSize: 65535},
			)
		}
	}

	dnsServer := server.New(
		servers,
		upchecks,
		time.Duration(config.BindTimeout),
		5*time.Second,
		shutdown,
		logger,
	)

	handlerRegistrar := handlers.NewHandlerRegistrar(logger, clock, recordSet, mux, nextInternalHandler)
	handlerRegistrar.RegisterAgentTLD()
	handlerRegistrar.UpdateDomainRegistrations()
	go func() {
		err := handlerRegistrar.Run(shutdown)
		if err != nil {
			logger.Error(logTag, fmt.Sprintf("could not start handler registrar: %s", err.Error()))
		}
	}()

	if metricsServerWrapper != nil {
		go func() {
			err := metricsServerWrapper.Run(shutdown)
			logger.Error(logTag, "could not start metric server: %s", err.Error())
		}()
	}

	go healthWatcher.Run(shutdown)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	go func() {
		<-sigterm
		close(repoUpdate)
		close(shutdown)
	}()

	jobs, err := healthconfig.ParseJobs(config.JobsDir, "")
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("failed to parse jobs directory: %s", err.Error()))
		return 1
	}

	http.Handle("/instances", api.NewInstancesHandler(recordSet, healthWatcher))
	http.Handle("/local-groups", api.NewLocalGroupsHandler(jobs, healthChecker))

	go func(config dnsconfig.APIConfig) {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithIdentityFromFile(config.CertificateFile, config.PrivateKeyFile),
			tlsconfig.WithInternalServiceDefaults(),
		).Server(
			tlsconfig.WithClientAuthenticationFromFile(config.CAFile),
		)
		if err != nil {
			log.Fatal(err)
			return
		}
		tlsConfig.BuildNameToCertificate()

		server := &http.Server{
			Addr:      fmt.Sprintf("127.0.0.1:%d", config.Port),
			TLSConfig: tlsConfig,
		}
		server.SetKeepAlivesEnabled(false)

		server.ListenAndServeTLS("", "")
	}(config.API)

	if err := dnsServer.Run(); err != nil {
		logger.Error(logTag, "bosh-dns failed: %s", err.Error())
		return 1
	}

	logger.Info(logTag, "bosh-dns stopped")
	return 0
}

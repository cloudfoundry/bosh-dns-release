package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
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
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/shuffle"
	"bosh-dns/healthconfig"
	"bosh-dns/tlsclient"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
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
	logTag := "main"
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
		quietLogger := boshlog.NewAsyncWriterLogger(boshlog.LevelNone, ioutil.Discard)
		httpClient, err := tlsclient.NewFromFiles("health.bosh-dns", config.Health.CAFile, config.Health.CertificateFile, config.Health.PrivateKeyFile, quietLogger)
		if err != nil {
			logger.Error(logTag, fmt.Sprintf("Unable to configure health checker %s", err.Error()))
			return 1
		}
		healthChecker = healthiness.NewHealthChecker(httpClient, config.Health.Port)
		checkInterval := time.Duration(config.Health.CheckInterval)
		healthWatcher = healthiness.NewHealthWatcher(1000, healthChecker, clock, checkInterval, logger)
	}

	shutdown := make(chan struct{})

	fileReader := records.NewFileReader(config.RecordsFile, system.NewOsFileSystem(logger), clock, logger, repoUpdate)
	filtererFactory := records.NewHealthFiltererFactory(healthWatcher)
	recordSet, err := records.NewRecordSet(fileReader, aliasConfiguration, healthWatcher, uint(config.Health.MaxTrackedQueries), shutdown, logger, filtererFactory, records.NewAliasEncoder())

	localDomain := dnsresolver.NewLocalDomain(logger, recordSet, shuffle.New())
	discoveryHandler := handlers.NewDiscoveryHandler(logger, localDomain)

	handlerRegistrar := handlers.NewHandlerRegistrar(logger, clock, recordSet, mux, discoveryHandler)

	recursorPool := handlers.NewFailoverRecursorPool(config.Recursors, config.RecursorSelection, logger)
	exchangerFactory := handlers.NewExchangerFactory(time.Duration(config.RecursorTimeout))
	forwardHandler := handlers.NewForwardHandler(recursorPool, exchangerFactory, clock, logger)

	mux.Handle("arpa.", handlers.NewRequestLoggerHandler(handlers.NewArpaHandler(logger, recordSet, forwardHandler), clock, logger))

	handlerFactory := handlers.NewFactory(exchangerFactory, clock, stringShuffler, logger)

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
		mux.Handle(upcheckDomain, handlers.NewUpcheckHandler(logger))
		for _, addr := range listenAddrs {
			upchecks = append(upchecks, server.NewDNSAnswerValidatingUpcheck(addr, upcheckDomain, "udp"))
			upchecks = append(upchecks, server.NewDNSAnswerValidatingUpcheck(addr, upcheckDomain, "tcp"))
		}
	}

	if config.Cache.Enabled {
		mux.Handle(".", handlers.NewCachingDNSHandler(forwardHandler))
	} else {
		mux.Handle(".", forwardHandler)
	}

	servers := []server.DNSServer{}
	numListeners := runtime.NumCPU()
	if runtime.GOOS == "windows" {
		numListeners = 1
	}

	for _, addr := range listenAddrs {
		for i := 0; i < numListeners; i++ {
			servers = append(servers,
				&dns.Server{Addr: addr, Net: "tcp", Handler: mux, ReusePort: true},
				&dns.Server{Addr: addr, Net: "udp", Handler: mux, ReusePort: true, UDPSize: 65535},
			)
		}
	}

	dnsServer := server.New(
		servers,
		upchecks,
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

	jobs, err := healthconfig.ParseJobs(config.JobsDir, "")
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("failed to parse jobs directory: %s", err.Error()))
		return 1
	}

	http.Handle("/instances", api.NewInstancesHandler(recordSet, healthWatcher))
	http.Handle("/local-groups", api.NewLocalGroupsHandler(jobs, healthChecker))

	go func(config dnsconfig.APIConfig) {
		caCert, err := ioutil.ReadFile(config.CAFile)
		if err != nil {
			log.Fatal(err)
			return
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		cert, err := tls.LoadX509KeyPair(config.CertificateFile, config.PrivateKeyFile)
		if err != nil {
			log.Fatal(err)
			return
		}

		tlsConfig := tlsconfig.Build(
			tlsconfig.WithIdentity(cert),
			tlsconfig.WithInternalServiceDefaults(),
		)

		serverConfig := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))
		serverConfig.BuildNameToCertificate()

		server := &http.Server{
			Addr:      fmt.Sprintf("127.0.0.1:%d", config.Port),
			TLSConfig: serverConfig,
		}
		server.SetKeepAlivesEnabled(false)

		server.ListenAndServeTLS("", "")
	}(config.API)

	if err := dnsServer.Run(); err != nil {
		logger.Error(logTag, err.Error())
		return 1
	}

	return 0
}

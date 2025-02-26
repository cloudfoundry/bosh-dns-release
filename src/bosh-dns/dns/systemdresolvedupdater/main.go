package main

import (
	"flag"
	"log"
	"time"

	"bosh-dns/dns/config"
	"bosh-dns/dns/config/handlers"
	"bosh-dns/dns/manager"
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/records"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	boshDnsConfig, err := config.LoadFromFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	level, err := boshDnsConfig.GetLogLevel()
	if err != nil {
		log.Fatal(err)
	}
	logr := logger.NewLogger(level)

	shutdown := make(chan struct{})
	fileReader := records.NewFileReader(boshDnsConfig.RecordsFile, system.NewOsFileSystem(logr), clock.NewClock(), logr, shutdown)
	fs := system.NewOsFileSystem(logr)
	healthWatcher := healthiness.NewNopHealthWatcher()

	aliasConfiguration, err := aliases.ConfigFromGlob(
		fs,
		aliases.NewFSLoader(fs),
		boshDnsConfig.AliasFilesGlob,
	)
	if err != nil {
		log.Fatal(err)
	}

	filtererFactory := records.NewHealthFiltererFactory(healthWatcher, time.Duration(boshDnsConfig.Health.SynchronousCheckTimeout))

	recordSet, err := records.NewRecordSet(fileReader, aliasConfiguration, healthWatcher, uint(boshDnsConfig.Health.MaxTrackedQueries), shutdown, logr, filtererFactory, records.NewAliasEncoder())
	if err != nil {
		log.Fatal(err)
	}

	handlersConfiguration, err := handlers.ConfigFromGlob(
		fs,
		handlers.NewFSLoader(fs),
		boshDnsConfig.HandlersFilesGlob,
	)
	if err != nil {
		log.Fatal(err)
	}

	if boshDnsConfig.ConfigureSystemdResolved {
		systemdResolvedManager := manager.NewSystemdResolvedManager(system.NewExecCmdRunner(logr))
		var domains []string
		domains = append(domains, handlersConfiguration.HandlerDomains()...)
		domains = append(domains, recordSet.Domains()...)
		err = systemdResolvedManager.UpdateDomains(domains)
		if err != nil {
			log.Fatal(err)
		}
	}
	close(shutdown)
}

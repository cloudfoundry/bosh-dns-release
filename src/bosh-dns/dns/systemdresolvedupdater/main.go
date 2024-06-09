package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"bosh-dns/dns/config"
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

	config, err := config.LoadFromFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	level, err := config.GetLogLevel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	logger := logger.NewLogger(level)

	clock := clock.NewClock()
	shutdown := make(chan struct{})
	fileReader := records.NewFileReader(config.RecordsFile, system.NewOsFileSystem(logger), clock, logger, shutdown)
	fs := system.NewOsFileSystem(logger)
	healthWatcher := healthiness.NewNopHealthWatcher()

	aliasConfiguration, err := aliases.ConfigFromGlob(
		fs,
		aliases.NewFSLoader(fs),
		config.AliasFilesGlob,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	filtererFactory := records.NewHealthFiltererFactory(healthWatcher, time.Duration(config.Health.SynchronousCheckTimeout))

	recordSet, err := records.NewRecordSet(fileReader, aliasConfiguration, healthWatcher, uint(config.Health.MaxTrackedQueries), shutdown, logger, filtererFactory, records.NewAliasEncoder())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if config.ConfigureSystemdResolved {
		systemdResolvedManager := manager.NewSystemdResolvedManager(system.NewExecCmdRunner(logger))
		err = systemdResolvedManager.UpdateDomains(recordSet.Domains())
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	close(shutdown)
}

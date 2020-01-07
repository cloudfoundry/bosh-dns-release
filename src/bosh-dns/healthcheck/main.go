package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bosh-dns/healthcheck/healthexecutable"
	"bosh-dns/healthcheck/healthserver"
	"bosh-dns/healthconfig"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var healthServer healthserver.HealthServer

type LinkJson struct {
	Group string `json:"group"`
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	const logTag = "healthcheck"

	config, err := getConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	logLevel := boshlog.LevelInfo
	if config.LogLevel != "" {
		logLevel, err = boshlog.Levelify(config.LogLevel)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
	}

	logger := boshlog.NewAsyncWriterLogger(logLevel, os.Stdout)
	defer logger.FlushTimeout(5 * time.Second)
	logger.Info(logTag, "Initializing")

	shutdown := make(chan struct{})

	cmdRunner := boshsys.NewExecCmdRunner(logger)
	interval := time.Duration(config.HealthExecutableInterval)

	jobs, err := healthconfig.ParseJobs(config.JobsDir, config.HealthExecutablePath)
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("failed parsing jobs: %v", err.Error()))
		return 1
	}
	logger.Info(logTag, fmt.Sprintf("Monitored jobs: %+v", jobs))

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	go func() {
		<-sigterm
		close(shutdown)
	}()

	healthExecutableMonitor := healthexecutable.NewMonitor(
		config.HealthFileName,
		jobs,
		cmdRunner,
		clock.NewClock(),
		interval,
		shutdown,
		logger,
	)

	healthServer = healthserver.NewHealthServer(logger, config.HealthFileName, healthExecutableMonitor, shutdown, time.Duration(config.RequestTimeout))
	healthServer.Serve(config)

	return 0
}

func getConfig() (*healthconfig.HealthCheckConfig, error) {
	var configFile string

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	} else {
		return nil, errors.New("Expected config file path argument")
	}

	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't open config file for health. error: %s", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	config := healthconfig.HealthCheckConfig{}
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode config file for health. error: %s", err)
	}

	return &config, nil
}

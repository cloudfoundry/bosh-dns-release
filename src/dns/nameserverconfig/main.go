package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/monitor"
)

func main() {
	var bindAddress string
	flag.StringVar(&bindAddress, "bindAddress", "", "address that our dns server is binding to")
	flag.Parse()

	if net.ParseIP(bindAddress) == nil {
		log.Fatalf("invalid ip: %s", bindAddress)
	}

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, os.Stdout, os.Stderr)
	defer logger.FlushTimeout(5 * time.Second)

	shutdown := make(chan struct{})
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	fs := boshsys.NewOsFileSystem(logger)
	clock := clock.NewClock()

	dnsManager := newDNSManager(logger, clock, fs)

	monitor := monitor.NewMonitor(
		logger,
		bindAddress,
		dnsManager,
		3*time.Second,
	)
	go monitor.Run(shutdown)

	func() {
		<-sigterm
		close(shutdown)
	}()
}

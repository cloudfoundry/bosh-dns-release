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

	"github.com/cloudfoundry/dns-release/src/dns/config"
	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/miekg/dns"
	"github.com/cloudfoundry/bosh-utils/logger"
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
	configPath, err := parseFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	c, err := config.LoadFromFile(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger := logger.NewLogger(logger.LevelDebug)
	mux := dns.NewServeMux()
	mux.Handle("healthcheck.bosh-dns.", handlers.NewHealthCheckHandler(logger))
	mux.Handle(".", handlers.NewForwardHandler(c.Recursors, handlers.NewExchangerFactory(time.Duration(c.RecursorTimeout)), logger))


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
		fmt.Println(err)

		os.Exit(1)
	}

	os.Exit(0)
}

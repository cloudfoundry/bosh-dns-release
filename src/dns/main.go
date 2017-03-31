package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/cloudfoundry/dns-release/src/dns/config"
	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/miekg/dns"
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

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	})

	bindAddress := fmt.Sprintf("%s:%d", c.Address, c.Port)
	tcpServer := &dns.Server{Addr: bindAddress, Net: "tcp"}
	udpServer := &dns.Server{Addr: bindAddress, Net: "udp", UDPSize: 65535}

	if err := server.New(tcpServer, udpServer, net.Dial, time.Duration(c.Timeout), bindAddress).ListenAndServe(); err != nil {
		fmt.Println(err)

		os.Exit(1)
	}
}

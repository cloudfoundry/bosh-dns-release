package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/cloudfoundry/dns-release/src/dns/config"
	"github.com/miekg/dns"
)

func createServer(protocol string, address string, port int, wg *sync.WaitGroup) error {
	defer wg.Done()

	server := &dns.Server{Addr: fmt.Sprintf("%s:%d", address, port), Net: protocol, UDPSize: 65535}
	return server.ListenAndServe()
}

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

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if createServer("tcp", c.DNS.Address, c.DNS.Port, &wg) != nil {
			os.Exit(1)
		}
	}()

	wg.Add(1)
	go func() {
		if createServer("udp", c.DNS.Address, c.DNS.Port, &wg) != nil {
			os.Exit(1)
		}
	}()

	wg.Wait()
}

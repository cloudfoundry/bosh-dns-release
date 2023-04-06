package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
)

func main() {
	domain := flag.String("checkDomain", "", "domain to lookup")
	timeout := flag.Duration("timeout", time.Minute, "amount of time to wait for check to pass")
	address := flag.String("address", "", "nameserver address")
	port := flag.Int("port", 53, "nameserver port")
	logFormat := flag.String("logFormat", "rfc3339", "log format")

	flag.Parse()

	bomb := time.NewTimer(*timeout)

	success := make(chan bool)

	log := logger.NewAsyncWriterLogger(logger.LevelDebug, os.Stdout)
	if *logFormat == "rfc3339" {
		log.UseRFC3339Timestamps()
	}

	resolver := getResolver(log, *address, *port)
	log.Info("wait", "resolving %s", *domain)

	go func() {
		for {
			tc, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			hosts, err := resolver.LookupHost(tc, *domain)
			if err == nil {
				log.Debug("wait", "%+v", hosts)
				success <- true
				return
			}
			log.Debug("wait", err.Error())
			time.Sleep(1 * time.Second)
		}
	}()

	select {
	case <-bomb.C:
		log.Error("wait", "timeout")
		log.FlushTimeout(5 * time.Second) //nolint:errcheck
		os.Exit(1)
	case <-success:
		log.Info("wait", "success")
		log.FlushTimeout(5 * time.Second) //nolint:errcheck
		os.Exit(0)
	}
}

func getResolver(log logger.Logger, address string, port int) *net.Resolver {
	if address != "" {
		nameserver := net.JoinHostPort(address, fmt.Sprintf("%d", port))
		log.Info("wait", "using nameserver %s", nameserver)
		dialFunc := func(c context.Context, network string, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(c, network, nameserver)
		}
		return &net.Resolver{
			PreferGo: true,
			Dial:     dialFunc,
		}
	} else {
		log.Info("wait", "using default resolver")
		return net.DefaultResolver
	}
}

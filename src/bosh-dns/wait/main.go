package main

import (
	"flag"
	"os"
	"time"

	"net"
)

func main() {
	domain := flag.String("checkDomain", "", "dns address to confirm command success")
	timeout := flag.Duration("timeout", time.Minute, "amount of time to wait for check to pass")

	flag.Parse()

	bomb := time.NewTimer(*timeout)

	success := make(chan bool)

	go func() {
		for {
			_, err := net.LookupHost(*domain)
			if err == nil {
				success <- true
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()

	select {
	case <-bomb.C:
		os.Exit(1)
	case <-success:
		os.Exit(0)
	}
}

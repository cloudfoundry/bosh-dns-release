package main

import (
	"github.com/miekg/dns"
	"os"
	"sync"
)

func createServer(protocol string, wg *sync.WaitGroup) {
	defer wg.Done()

	server := &dns.Server{Addr: "127.0.0.1:9955", Net: protocol, UDPSize: 65535}
	err := server.ListenAndServe()
	if err != nil {
		os.Exit(1)
	}
}

func main() {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	})

	wg := sync.WaitGroup{}
	wg.Add(2)

	go createServer("tcp", &wg)
	go createServer("udp", &wg)

	wg.Wait()
}

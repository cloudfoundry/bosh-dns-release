package main

import (
	"github.com/miekg/dns"
	"os"
)

func main() {
	server := &dns.Server{Addr: "127.0.0.1:9955", Net: "tcp"}

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	})

	if err := server.ListenAndServe(); err != nil {
		os.Exit(1)
	}
}

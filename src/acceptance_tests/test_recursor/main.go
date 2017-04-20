package main

import (
	"net"
	"os"

	"github.com/miekg/dns"
)

func main() {
	server := &dns.Server{Addr: "0.0.0.0:9955", Net: "udp"}
	dns.HandleFunc("truncated-recursor.com.", func(w dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("9.9.9.9"),
		})

		msg.Authoritative = true
		msg.RecursionAvailable = false
		msg.Truncated = true

		msg.SetReply(req)
		w.WriteMsg(msg)
	})

	dns.HandleFunc("example.com.", func(w dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("10.10.10.10"),
		})

		msg.Authoritative = true
		msg.RecursionAvailable = false

		msg.SetReply(req)
		w.WriteMsg(msg)
	})

	if err := server.ListenAndServe(); err != nil {
		os.Exit(1)
	}
}

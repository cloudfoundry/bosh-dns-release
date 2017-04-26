package main

import (
	"net"
	"os"

	"fmt"
	"github.com/miekg/dns"
)

func main() {
	server := &dns.Server{Addr: "0.0.0.0:9955", Net: "udp", UDPSize: 65535}
	dns.HandleFunc("truncated-recursor.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
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
		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("udp-9k-message.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.Compress = false

		if _, ok := resp.RemoteAddr().(*net.TCPAddr); ok {
			msg.SetRcode(req, dns.RcodeServerFailure)
		} else {
			msg.SetRcode(req, dns.RcodeSuccess)
		}

		m1 := 2048
		for i := 0; i < m1; i++ {
			aRec := &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(fmt.Sprintf("127.0.0.%d", i+1)).To4(),
			}

			msg.Answer = append(msg.Answer, aRec)
		}

		maxUdpSize := 9216
		for len(msg.Answer) > 0 && msg.Len() > maxUdpSize {
			msg.Answer = msg.Answer[:len(msg.Answer)-1]
		}

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("ip-truncated-recursor-large.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.SetRcode(req, dns.RcodeSuccess)
		msg.Authoritative = true

		m1 := 512
		for i := 0; i < m1; i++ {
			aRec := &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(fmt.Sprintf("127.0.0.%d", i+1)).To4(),
			}

			msg.Answer = append(msg.Answer, aRec)
		}

		maxUdpSize := 1024
		for len(msg.Answer) > 0 && msg.Len() > maxUdpSize {
			msg.Answer = msg.Answer[:len(msg.Answer)-1]
		}

		msg.Truncated = true
		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("recursor-small.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.SetRcode(req, dns.RcodeSuccess)
		msg.Authoritative = true
		m1 := 2
		for i := 0; i < m1; i++ {
			aRec := &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(fmt.Sprintf("127.0.0.%d", i+1)).To4(),
			}

			msg.Answer = append(msg.Answer, aRec)
		}

		msg.Truncated = false

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("compressed-ip-truncated-recursor-large.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.SetRcode(req, dns.RcodeSuccess)
		msg.Authoritative = true
		msg.Compress = true

		m1 := 512
		for i := 0; i < m1; i++ {
			aRec := &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(fmt.Sprintf("127.0.0.%d", i+1)).To4(),
			}

			msg.Answer = append(msg.Answer, aRec)
		}

		maxUdpSize := 9216
		for len(msg.Answer) > 0 && msg.Len() > maxUdpSize {
			msg.Answer = msg.Answer[:len(msg.Answer)-1]
		}

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("example.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
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

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	if err := server.ListenAndServe(); err != nil {
		os.Exit(1)
	}
}

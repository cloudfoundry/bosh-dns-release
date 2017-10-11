package main

import (
	"net"
	"os"

	"fmt"
	"strconv"
	"time"

	"github.com/miekg/dns"
)

func main() {
	server := &dns.Server{Addr: fmt.Sprintf("0.0.0.0:%d", getRecursorPort()), Net: "udp", UDPSize: 65535}
	nextAddress := 0

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
		msg.RecursionAvailable = true
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
		msg.RecursionAvailable = true
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
		msg.RecursionAvailable = true

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
		msg.RecursionAvailable = true

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

	dns.HandleFunc("slow-recursor.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.SetRcode(req, dns.RcodeSuccess)
		msg.Authoritative = true
		msg.RecursionAvailable = true

		aRec := &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("127.0.0.1").To4(),
		}
		msg.Answer = append(msg.Answer, aRec)

		time.Sleep(3 * time.Second)
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
		msg.RecursionAvailable = true
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
		msg.RecursionAvailable = true

		msg.SetReply(req)

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("always-different-with-timeout-example.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    5,
			},
			A: net.ParseIP(fmt.Sprintf("127.0.0.%d", nextAddress+1)).To4(),
		})
		nextAddress = nextAddress + 1

		msg.Authoritative = true
		msg.RecursionAvailable = true

		msg.SetReply(req)

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("Unable to start server: error: +%v", err)
		os.Exit(1)
	}
}

func getRecursorPort() int {
	port := 9955
	if len(os.Args) >= 2 {
		var err error
		port, err = strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Printf("Could not determine server port: error: +%v", err)
			os.Exit(1)
		}
	}
	return port
}

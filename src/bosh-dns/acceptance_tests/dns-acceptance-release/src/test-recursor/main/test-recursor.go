package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"

	"test-recursor/config"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config file>\n", os.Args[0])
		os.Exit(1)
	}

	cfg := loadConfig(os.Args[1])

	server := &dns.Server{Addr: fmt.Sprintf("0.0.0.0:%d", cfg.Port), Net: "udp", UDPSize: 65535}
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

	dns.HandleFunc("alternating-slow-recursor.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
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
				Ttl:    5,
			},
			A: net.ParseIP("127.0.0.1").To4(),
		}
		msg.Answer = append(msg.Answer, aRec)

		if msg.Id%2 == 1 {
			time.Sleep(2 * time.Second)
		}
		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("alternating-nameerror-recursor.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.RecursionAvailable = true

		if msg.Id%2 == 1 {
			msg.SetRcode(req, dns.RcodeNameError)
		} else {
			msg.SetRcode(req, dns.RcodeSuccess)
			aRec := &dns.A{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: req.Question[0].Qtype,
					Class:  dns.ClassINET,
					Ttl:    5,
				},
				A: net.ParseIP("127.0.0.1").To4(),
			}
			msg.Answer = append(msg.Answer, aRec)
		}

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("alternating-soa-nameerror-recursor.com.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = true
		msg.RecursionAvailable = true

		if msg.Id%2 == 1 {
			msg.SetRcode(req, dns.RcodeNameError)
		} else {
			msg.SetRcode(req, dns.RcodeSuccess)
			soaRec := &dns.SOA{
				Hdr: dns.RR_Header{
					Name:   req.Question[0].Name,
					Rrtype: req.Question[0].Qtype,
					Class:  dns.ClassINET,
					Ttl:    5,
				},
				Ns:      "ns1.pivotal.io",
				Mbox:    "postmaster.pivotal.io",
				Serial:  1987,
				Refresh: 3600,
				Retry:   600,
				Expire:  604800,
				Minttl:  3600,
			}
			msg.Answer = append(msg.Answer, soaRec)
		}

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

		for i := 0; msg.Len() < 512; i++ { // uncompressed length just over 512
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
		msg.Compress = true // compressed size should be < 512

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

	dns.HandleFunc("handler.internal.local.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("10.168.0.1"),
		})

		msg.Authoritative = true
		msg.RecursionAvailable = true

		msg.SetReply(req)

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("4.4.8.8.in-addr.arpa.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			Ptr: "google-public-dns-b.google.com.",
		})

		msg.Authoritative = true
		msg.RecursionAvailable = true

		msg.SetReply(req)

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	dns.HandleFunc("8.8.8.8.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.ip6.arpa.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			Ptr: "google-public-dns-a.google.com.",
		})

		msg.Authoritative = true
		msg.RecursionAvailable = true

		msg.SetReply(req)

		err := resp.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	})

	// This handles the question whose response can be defined in the config. We
	// will use this when we are testing the order of the upstream recursors
	// (shuffled or serial).
	dns.HandleFunc("question_with_configurable_response.", func(resp dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   req.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(cfg.ConfigurableResponse),
		})

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

func loadConfig(filename string) *config.Config {
	cfg := config.NewConfig()
	if err := cfg.LoadFromFile(filename); err != nil {
		fmt.Fprintf(os.Stderr, "could not read config file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	return cfg
}

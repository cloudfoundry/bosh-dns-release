package internal_test

import (
	"net"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/internal"
)

var _ = Describe("RequestLoggerHandler", func() {
	var (
		fakeLogger  *loggerfakes.FakeLogger
		handler     handlers.RequestLoggerHandler
		child       dns.Handler
		fakeClock   *fakeclock.FakeClock
		dnsRequest  *dns.Msg
		dnsResponse *dns.Msg

		makeHandler func() dns.Handler
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		fakeClock = fakeclock.NewFakeClock(time.Now())

		makeHandler = func() dns.Handler {
			return dns.HandlerFunc(func(resp dns.ResponseWriter, req *dns.Msg) {
				m := &dns.Msg{}
				m.Authoritative = true
				m.RecursionAvailable = false

				fakeClock.Increment(time.Nanosecond * 3)

				Expect(resp.WriteMsg(m)).To(Succeed())
			})
		}
		dnsRequest = &dns.Msg{
			Question: []dns.Question{
				{Name: "q-what.bosh.", Qtype: dns.TypeA},
			},
		}
		dnsRequest.Id = 123
		dnsResponse = &dns.Msg{
			Question: []dns.Question{
				{Name: "q-what.bosh.", Qtype: dns.TypeA},
			},
			Answer: []dns.RR{},
		}
		child = makeHandler()
		handler = handlers.NewRequestLoggerHandler(child, fakeClock, fakeLogger)

	})

	Describe("log request", func() {

		Context("when a request is received", func() {
			It("logs a debug message", func() {
				internal.LogReceivedRequest(fakeLogger, handler, "mytag", dnsRequest)

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				Expect(message).To(Equal("handlers.RequestLoggerHandler Received request id=123 qtype=[A] qname=[q-what.bosh.]"))
			})
		})

		Context("when passed a successful rcode", func() {
			It("logs a debug message", func() {
				dnsResponse.Rcode = dns.RcodeSuccess
				dnsResponse.Answer = append(dnsResponse.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   dnsRequest.Question[0].Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    0,
					},
					A: net.ParseIP("127.0.0.1"),
				})
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsRequest, dnsResponse, "")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				Expect(message).To(Equal("handlers.RequestLoggerHandler Request id=123 qtype=[A] qname=[q-what.bosh.] rcode=NOERROR ancount=1 time=1ns"))
			})
		})

		Context("when passed a name error rcode", func() {
			It("logs a debug message", func() {
				dnsResponse.Rcode = dns.RcodeNameError
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsRequest, dnsResponse, "")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				Expect(message).To(Equal("handlers.RequestLoggerHandler Request id=123 qtype=[A] qname=[q-what.bosh.] rcode=NXDOMAIN ancount=0 time=1ns"))
			})
		})

		Context("when passed a custom message", func() {
			It("adds it to the log message", func() {
				dnsResponse.Rcode = dns.RcodeSuccess
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsRequest, dnsResponse, "custom-message")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				Expect(message).To(Equal("handlers.RequestLoggerHandler Request id=123 qtype=[A] qname=[q-what.bosh.] rcode=NOERROR ancount=0 custom-message time=1ns"))
			})
		})

		Context("when passed a nil response", func() {
			It("logs SRVFAIL", func() {
				dnsResponse.Rcode = dns.RcodeNameError
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsRequest, nil, "")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				Expect(message).To(Equal("handlers.RequestLoggerHandler Request id=123 qtype=[A] qname=[q-what.bosh.] rcode=SERVFAIL ancount=0 time=1ns"))
			})
		})
	})
})

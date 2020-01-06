package internal_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/internal"
	"fmt"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/miekg/dns"

	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RequestLoggerHandler", func() {
	var (
		fakeLogger *loggerfakes.FakeLogger
		handler    handlers.RequestLoggerHandler
		child      dns.Handler
		fakeClock  *fakeclock.FakeClock
		dnsMsg     *dns.Msg

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
		dnsMsg = &dns.Msg{
			Question: []dns.Question{
				{Name: "q-what.bosh.", Qtype: dns.TypeA},
			},
		}
		child = makeHandler()
		handler = handlers.NewRequestLoggerHandler(child, fakeClock, fakeLogger)

	})

	Describe("log request", func() {
		Context("when passed a successful rcode", func() {
			It("logs a debug message", func() {
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsMsg, 0, "")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				fmt.Println(message)
				Expect(message).To(Equal("handlers.RequestLoggerHandler Request qtype=[A] qname=[q-what.bosh.] rcode=NOERROR time=1ns"))
			})
		})

		Context("when passed a failing rcode", func() {
			It("logs a warn message", func() {
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsMsg, dns.RcodeServerFailure, "")

				Expect(fakeLogger.WarnCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.WarnArgsForCall(0)

				fmt.Println(message)
				Expect(message).To(Equal("handlers.RequestLoggerHandler Request qtype=[A] qname=[q-what.bosh.] rcode=SERVFAIL time=1ns"))
			})
		})

		Context("when passed a not implemented rcode", func() {
			It("logs a debug message", func() {
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsMsg, dns.RcodeNotImplemented, "")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				fmt.Println(message)
				Expect(message).To(Equal("handlers.RequestLoggerHandler Request qtype=[A] qname=[q-what.bosh.] rcode=NOTIMP time=1ns"))
			})
		})

		Context("when passed a custom message", func() {
			It("adds it to the log message", func() {
				internal.LogRequest(fakeLogger, handler, "mytag", 1, dnsMsg, 0, "custom-message")

				Expect(fakeLogger.DebugCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.DebugArgsForCall(0)

				fmt.Println(message)
				Expect(message).To(Equal("handlers.RequestLoggerHandler Request qtype=[A] qname=[q-what.bosh.] rcode=NOERROR custom-message time=1ns"))
			})
		})
	})
})

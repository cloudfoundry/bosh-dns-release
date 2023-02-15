package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/miekg/dns"

	"time"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RequestLoggerHandler", func() {
	var (
		fakeLogger        *loggerfakes.FakeLogger
		handler           handlers.RequestLoggerHandler
		child             dns.Handler
		dispatchedRequest dns.Msg
		fakeWriter        *internalfakes.FakeResponseWriter
		fakeClock         *fakeclock.FakeClock

		makeHandler func(int) dns.Handler
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		fakeClock = fakeclock.NewFakeClock(time.Now())

		makeHandler = func(rcode int) dns.Handler {
			return dns.HandlerFunc(func(resp dns.ResponseWriter, req *dns.Msg) {
				dispatchedRequest = *req

				m := &dns.Msg{}
				m.Authoritative = true
				m.RecursionAvailable = false
				m.SetRcode(req, rcode)

				fakeClock.Increment(time.Nanosecond * 3)

				Expect(resp.WriteMsg(m)).To(Succeed())
			})
		}

		child = makeHandler(dns.RcodeSuccess)

		handler = handlers.NewRequestLoggerHandler(child, fakeClock, fakeLogger)
	})

	Describe("ServeDNS", func() {
		It("delegates to the child handler", func() {
			m := dns.Msg{}
			SetQuestion(&m, nil, "upcheck.bosh-dns.", dns.TypeANY)

			handler.ServeDNS(fakeWriter, &m)

			Expect(dispatchedRequest).To(Equal(m))

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(message.Authoritative).To(Equal(true))
			Expect(message.RecursionAvailable).To(Equal(false))
		})

		It("logs the request info", func() {
			var casedQname string
			m := &dns.Msg{}
			SetQuestion(m, &casedQname, "upcheck.bosh-dns.", dns.TypeANY)
			m.Id = 123

			handler.ServeDNS(fakeWriter, m)

			Expect(fakeLogger.DebugCallCount()).To(Equal(2))
			tag, message, _ := fakeLogger.DebugArgsForCall(0)
			Expect(tag).To(Equal("RequestLoggerHandler"))
			Expect(message).To(Equal("dns.HandlerFunc Received request id=123 qtype=[ANY] qname=[" + casedQname + "]"))

			tag, message, _ = fakeLogger.DebugArgsForCall(1)
			Expect(tag).To(Equal("RequestLoggerHandler"))
			Expect(message).To(Equal("dns.HandlerFunc Request id=123 qtype=[ANY] qname=[" + casedQname + "] rcode=NOERROR ancount=0 time=3ns"))
		})
	})
})

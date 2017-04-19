package handlers_test

import (
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/clock/clockfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("RequestLoggerHandler", func() {
	var (
		fakeLogger        *loggerfakes.FakeLogger
		handler           handlers.RequestLoggerHandler
		child             dns.Handler
		dispatchedRequest dns.Msg
		fakeWriter        *internalfakes.FakeResponseWriter
		fakeClock         *clockfakes.FakeClock

		makeHandler func(int) dns.Handler
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		fakeClock = &clockfakes.FakeClock{}

		now := time.Now()
		fakeClock.NowReturns(now)

		makeHandler = func(rcode int) dns.Handler {
			return dns.HandlerFunc(func(resp dns.ResponseWriter, req *dns.Msg) {
				dispatchedRequest = *req

				m := &dns.Msg{}
				m.Authoritative = true
				m.RecursionAvailable = false
				m.SetRcode(req, rcode)

				fakeClock.NowReturns(now.Add(time.Duration(time.Nanosecond * 3)))

				Expect(resp.WriteMsg(m)).ToNot(HaveOccurred())
			})
		}

		child = makeHandler(dns.RcodeSuccess)

		handler = handlers.NewRequestLoggerHandler(child, fakeClock, fakeLogger)
	})

	Describe("ServeDNS", func() {
		It("delegates to the child handler", func() {
			m := dns.Msg{}
			m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)

			handler.ServeDNS(fakeWriter, &m)

			Expect(dispatchedRequest).To(Equal(m))

			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(message.Authoritative).To(Equal(true))
			Expect(message.RecursionAvailable).To(Equal(false))
		})

		It("logs the request info", func() {
			m := &dns.Msg{}
			m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)

			handler.ServeDNS(fakeWriter, m)

			Expect(fakeLogger.InfoCallCount()).To(Equal(1))
			tag, message, _ := fakeLogger.InfoArgsForCall(0)
			Expect(tag).To(Equal("RequestLoggerHandler"))
			Expect(message).To(Equal("dns.HandlerFunc Request [255] [healthcheck.bosh-dns.] 0 3ns"))
		})

		Context("when there are no questions", func() {
			It("indicates empty question types array", func() {
				m := &dns.Msg{}

				handler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.InfoArgsForCall(0)
				Expect(message).To(Equal("dns.HandlerFunc Request [] [] 0 3ns"))
			})
		})

		Context("when there are multiple questions", func() {
			It("includes all question types in the log", func() {
				m := &dns.Msg{
					Question: []dns.Question{
						{Name: "healthcheck.bosh-dns.", Qtype: dns.TypeANY},
						{Name: "q-what.bosh.", Qtype: dns.TypeA},
					},
				}

				handler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.InfoArgsForCall(0)
				Expect(message).To(Equal("dns.HandlerFunc Request [255,1] [healthcheck.bosh-dns.,q-what.bosh.] 0 3ns"))
			})
		})

		Context("when the child handler serves RcodeFailure", func() {
			It("logs the rcode correctly", func() {
				child = makeHandler(dns.RcodeServerFailure)
				handler = handlers.NewRequestLoggerHandler(child, fakeClock, fakeLogger)

				m := &dns.Msg{
					Question: []dns.Question{
						{Name: "q-what.bosh.", Qtype: dns.TypeA},
					},
				}

				handler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
				_, message, _ := fakeLogger.InfoArgsForCall(0)
				Expect(message).To(Equal("dns.HandlerFunc Request [1] [q-what.bosh.] 2 3ns"))
			})
		})
	})
})

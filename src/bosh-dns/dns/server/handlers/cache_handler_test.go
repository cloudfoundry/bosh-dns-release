package handlers_test

import (
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"time"
)

var _ bool = Describe("CacheHandler", func() {
	var (
		cacheHandler   handlers.CachingDNSHandler
		fakeWriter     *internalfakes.FakeResponseWriter
		fakeDnsHandler *handlersfakes.FakeDNSHandler
		fakeTruncater  *dnsresolverfakes.FakeResponseTruncater
		fakeClock      *fakeclock.FakeClock
		fakeLogger     *loggerfakes.FakeLogger
		response       *dns.Msg
	)

	BeforeEach(func() {
		fakeDnsHandler = &handlersfakes.FakeDNSHandler{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		fakeTruncater = &dnsresolverfakes.FakeResponseTruncater{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeLogger = &loggerfakes.FakeLogger{}
		cacheHandler = handlers.NewCachingDNSHandler(fakeDnsHandler, fakeTruncater, fakeClock, fakeLogger)

		response = &dns.Msg{
			Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
		}
		response.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
		fakeDnsHandler.ServeDNSStub = func(cacheWriter dns.ResponseWriter, r *dns.Msg) {
			response.SetRcode(r, dns.RcodeSuccess)
			cacheWriter.WriteMsg(response)
		}
	})

	Describe("ServeDNS", func() {
		Context("when the request doesn't have recursion desired bit set", func() {
			It("forwards the question up to a recursor", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				m.RecursionDesired = false
				cacheHandler.ServeDNS(fakeWriter, m)
				Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
				_, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
				Expect(forwardedMsg).To(Equal(m))
			})

			It("truncates the recursor response if needed", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				m.RecursionDesired = false
				cacheHandler.ServeDNS(fakeWriter, m)
				Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(1))
				response := fakeWriter.WriteMsgArgsForCall(0)
				writer, req, resp := fakeTruncater.TruncateIfNeededArgsForCall(0)
				Expect(writer).To(Equal(fakeWriter))
				Expect(req).To(Equal(m))
				Expect(resp).To(Equal(response))
			})
		})

		Context("when the request doesn't have recursion desired bit set", func() {
			Context("when the answer is not cached", func() {
				It("forwards the question up to a recursor", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					m.RecursionDesired = true
					cacheHandler.ServeDNS(fakeWriter, m)
					Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
					_, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
					Expect(forwardedMsg.Question).To(Equal(m.Question))
				})

				It("caches the response", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					cacheHandler.ServeDNS(fakeWriter, m)

					Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					Expect(fakeWriter.WriteMsgArgsForCall(0)).To(Equal(response))
				})
			})

			Context("when an answer is cached", func() {
				BeforeEach(func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					cacheHandler.ServeDNS(fakeWriter, m) // should cache response
				})

				It("truncates the cached response if needed", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					m.RecursionDesired = false
					cacheHandler.ServeDNS(fakeWriter, m)
					Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(2))
					response := fakeWriter.WriteMsgArgsForCall(1)
					writer, req, resp := fakeTruncater.TruncateIfNeededArgsForCall(1)
					Expect(writer).To(Equal(fakeWriter))
					Expect(req).To(Equal(m))
					Expect(resp).To(Equal(response))
				})
			})
		})
	})
})

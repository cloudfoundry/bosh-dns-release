package handlers_test

import (
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
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
		SetQuestion(response, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
		fakeDnsHandler.ServeDNSStub = func(cacheWriter dns.ResponseWriter, r *dns.Msg) {
			response.SetRcode(r, dns.RcodeSuccess)
			Expect(cacheWriter.WriteMsg(response)).To(Succeed())
		}
	})

	Describe("ServeDNS", func() {
		Context("when the request doesn't have recursion desired bit set", func() {
			It("forwards the question up to a recursor", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				m.RecursionDesired = false
				cacheHandler.ServeDNS(fakeWriter, m)
				Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
				_, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
				Expect(forwardedMsg).To(Equal(m))
			})

			It("truncates the recursor response if needed", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
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
					SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					m.RecursionDesired = true
					cacheHandler.ServeDNS(fakeWriter, m)
					Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
					_, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
					Expect(forwardedMsg.Question).To(Equal(m.Question))
				})

				It("caches the response", func() {
					m := &dns.Msg{}
					SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					cacheHandler.ServeDNS(fakeWriter, m)

					Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					Expect(fakeWriter.WriteMsgArgsForCall(0)).To(Equal(response))
				})
			})

			Context("when an answer is cached", func() {
				BeforeEach(func() {
					m := &dns.Msg{}
					SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
					cacheHandler.ServeDNS(fakeWriter, m) // should cache response
				})

				It("truncates the cached response if needed", func() {
					m := &dns.Msg{}
					SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
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

	Context("when the response has the NXDOMAIN response code", func() {
		BeforeEach(func() {
			// We need an SOA along with an NXDOMAIN response, or it'll be processed as
			// an OTHERERROR, and will have a 5 second cache.
			// With the SOA, we're processed as an NXDOMAIN and -for better or worse-
			// use the TTL in the SOA as the TTL for the NXDOMAIN reply.
			response = &dns.Msg{
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "my-group.my-network.my-deployment.bosh.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    600,
						},
						Serial: 1234,
					},
				},
			}

			fmt.Printf("NS ARRAY LEN: %d\n", len(response.Ns))
			fmt.Printf("RESPONSE '%s'\n", response.String())
			SetQuestion(response, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
			fakeDnsHandler.ServeDNSStub = func(cacheWriter dns.ResponseWriter, r *dns.Msg) {
				response.SetRcode(r, dns.RcodeNameError)
				fmt.Printf("RESPONSE inside servednsstub '%s'\n", response.String())
				Expect(cacheWriter.WriteMsg(response)).To(Succeed())
			}
		})
		Context("when the answer is not cached", func() {
			// It("forwards the question up to a recursor", func() {
			// 	m := &dns.Msg{}
			// 	SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
			// 	m.RecursionDesired = true
			// 	cacheHandler.ServeDNS(fakeWriter, m)
			// 	Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
			// 	_, forwardedMsg := fakeDnsHandler.ServeDNSArgsForCall(0)
			// 	Expect(forwardedMsg.Question).To(Equal(m.Question))
			// })

			It("actually caches the response", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				cacheHandler.ServeDNS(fakeWriter, m)

				Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
				Expect(fakeWriter.WriteMsgArgsForCall(0)).To(Equal(response))

				// Call the function again to make sure it seems like we fetch from cache:
				cacheHandler.ServeDNS(fakeWriter, m)
				// We don't call our fetching function again, because the data's in cache, so
				// we don't need to re-fetch it.
				Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
				// But we do call our "return the response" function again
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(2))
				Expect(fakeWriter.WriteMsgArgsForCall(0)).To(Equal(response))
			})
		})

		// Context("when an answer is cached", func() {
		// 	BeforeEach(func() {
		// 		m := &dns.Msg{}
		// 		SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
		// 		cacheHandler.ServeDNS(fakeWriter, m) // should cache response
		// 		Expect(fakeDnsHandler.ServeDNSCallCount()).To(Equal(1))
		// 	})

		// 	It("truncates the cached response if needed", func() {
		// 		m := &dns.Msg{}
		// 		SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
		// 		m.RecursionDesired = false
		// 		cacheHandler.ServeDNS(fakeWriter, m)
		// 		Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(2))
		// 		response := fakeWriter.WriteMsgArgsForCall(1)
		// 		writer, req, resp := fakeTruncater.TruncateIfNeededArgsForCall(1)
		// 		Expect(writer).To(Equal(fakeWriter))
		// 		Expect(req).To(Equal(m))
		// 		Expect(resp).To(Equal(response))
		// 	})
		// })
	})
})

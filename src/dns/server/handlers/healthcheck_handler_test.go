package handlers_test

import (
	"errors"

	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/internal/internalfakes"
	"github.com/miekg/dns"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
)

var _ = Describe("HealthCheckHandler", func() {
	var (
		fakeLogger         *loggerfakes.FakeLogger
		healthCheckHandler handlers.HealthCheckHandler
		fakeWriter         *internalfakes.FakeResponseWriter
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		healthCheckHandler = handlers.NewHealthCheckHandler(fakeLogger)
		fakeWriter = &internalfakes.FakeResponseWriter{}
	})

	Describe("ServeDNS", func() {
		It("returns success rcode", func() {
			m := &dns.Msg{}
			m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)

			healthCheckHandler.ServeDNS(fakeWriter, m)
			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(message.Authoritative).To(Equal(true))
			Expect(message.RecursionAvailable).To(Equal(false))
			Expect(len(message.Answer)).To(Equal(1))
			Expect(message.Answer[0]).To(Equal(&dns.A{
				Hdr: dns.RR_Header{
					Name:   "healthcheck.bosh-dns.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.IPv4(127, 0, 0, 1),
			}))
		})

		Context("when the message fails to write", func() {
			It("logs the error", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				m := &dns.Msg{}
				m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)
				healthCheckHandler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("HealthCheckHandler"))
				Expect(msg).To(Equal("failed to write message"))
			})
		})
	})
})

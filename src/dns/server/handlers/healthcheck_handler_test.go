package handlers_test

import (
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthCheckHandler", func() {
	Describe("ServeDNS", func() {
		It("returns success rcode", func() {
			healthCheckHandler := handlers.NewHealthCheckHandler()
			fakeWriter := &internalfakes.FakeResponseWriter{}

			m := &dns.Msg{}
			m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)

			healthCheckHandler.ServeDNS(fakeWriter, m)
			message := fakeWriter.WriteMsgArgsForCall(0)
			Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
			Expect(message.Authoritative).To(Equal(true))
			Expect(message.RecursionAvailable).To(Equal(false))
		})
	})
})

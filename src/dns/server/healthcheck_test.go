package server_test

import (
	"fmt"
	"math/rand"

	boshlogf "github.com/cloudfoundry/bosh-utils/logger/fakes"
	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func startServer(network string, address string, handler dns.Handler) dns.Server {
	notifyDone := make(chan bool)
	server := dns.Server{Addr: address, Net: network, Handler: handler, NotifyStartedFunc: func() {
		notifyDone <- true
	}}
	go func() {
		defer GinkgoRecover()
		err := server.ListenAndServe()

		// NOTE because of this issue https://github.com/miekg/dns/issues/457
		//   we will experience a non-graceful shutdown when terminating the
		//   TCP server. We have to allow that particular error.
		Expect(err).To(Or(Not(HaveOccurred()), MatchError(ContainSubstring("use of closed network connection"))))
	}()
	Expect(<-notifyDone).To(BeTrue())
	return server
}

var _ = Describe("Healthcheck", func() {
	var subject server.HealthCheck
	var dnsHandler dns.Handler
	var udpServer dns.Server
	var tcpServer dns.Server
	var listenDomain string
	var ports map[string]int
	var addresses map[string]string
	healthCheckDomain := "healthcheck.bosh-dns."

	JustBeforeEach(func() {
		ports["udp"] = rand.Int() % 50000
		ports["tcp"] = ports["udp"] + 1
		addresses["udp"] = fmt.Sprintf("%s:%d", listenDomain, ports["udp"])
		addresses["tcp"] = fmt.Sprintf("%s:%d", listenDomain, ports["tcp"])

		udpServer = startServer("udp", addresses["udp"], dnsHandler)
		tcpServer = startServer("tcp", addresses["tcp"], dnsHandler)
	})

	AfterEach(func() {
		err := tcpServer.Shutdown()
		Expect(err).NotTo(HaveOccurred())
		err = udpServer.Shutdown()
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		ports = map[string]int{}
		addresses = map[string]string{}
		listenDomain = "127.0.0.1"
		dnsHandler = handlers.NewHealthCheckHandler(&boshlogf.FakeLogger{})
	})

	Context("when the health check target is a malformed address", func() {
		DescribeTable("returns an error", func(network string) {
			subject = server.NewAnswerValidatingHealthCheck("~~~~~~~~~~", healthCheckDomain, network)

			err := subject.IsHealthy()
			Expect(err.Error()).To(Equal(fmt.Sprintf("on %s: missing port in address ~~~~~~~~~~", network)))
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the target server resolves the healthcheck domain", func() {
		Context("when the target address is 127.0.0.1", func() {
			DescribeTable("it checks on 127.0.0.1", func(network string) {
				subject = server.NewAnswerValidatingHealthCheck(fmt.Sprintf("127.0.0.1:%d", ports[network]), healthCheckDomain, network)

				err := subject.IsHealthy()
				Expect(err).NotTo(HaveOccurred())
			},
				Entry("when networking is udp", "udp"),
				Entry("when networking is tcp", "tcp"),
			)
		})

		Context("when the target address is 0.0.0.0", func() {
			DescribeTable("it checks on 127.0.0.1", func(network string) {
				subject = server.NewAnswerValidatingHealthCheck(fmt.Sprintf("0.0.0.0:%d", ports[network]), healthCheckDomain, network)

				err := subject.IsHealthy()
				Expect(err).NotTo(HaveOccurred())
			},
				Entry("when networking is udp", "udp"),
				Entry("when networking is tcp", "tcp"),
			)
		})
	})

	Context("when the health check takes a long time", func() {
		DescribeTable("times out with error", func(network string) {
			subject = server.NewAnswerValidatingHealthCheck("10.10.10.10:30", healthCheckDomain, network)

			err := subject.IsHealthy()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp(`on %s:.*%s.*10\.10\.10\.10.*i/o timeout`, network, network))
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the health check domain resolves with no answers", func() {
		BeforeEach(func() {
			dnsHandler = dns.HandlerFunc(func(r dns.ResponseWriter, m *dns.Msg) {
				m.Rcode = dns.RcodeSuccess
				r.WriteMsg(m)
			})
		})

		DescribeTable("returns with error", func(network string) {
			subject = server.NewAnswerValidatingHealthCheck(addresses[network], healthCheckDomain, network)

			err := subject.IsHealthy()
			Expect(err).To(HaveOccurred())
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the health check domain resolve failed", func() {
		BeforeEach(func() {
			dnsHandler = dns.HandlerFunc(func(r dns.ResponseWriter, m *dns.Msg) {
				m.Rcode = dns.RcodeServerFailure
				r.WriteMsg(m)
			})
		})

		DescribeTable("returns with error", func(network string) {
			subject = server.NewAnswerValidatingHealthCheck(addresses[network], healthCheckDomain, network)

			err := subject.IsHealthy()
			Expect(err).To(HaveOccurred())
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})
})

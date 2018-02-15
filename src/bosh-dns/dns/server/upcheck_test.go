package server_test

import (
	"fmt"

	boshlogf "github.com/cloudfoundry/bosh-utils/logger/fakes"
	"github.com/miekg/dns"

	"bosh-dns/dns/internal/testhelpers"
	"bosh-dns/dns/server"
	"bosh-dns/dns/server/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func startServer(network string, address string, handler dns.Handler) *dns.Server {
	notifyDone := make(chan struct{})
	server := &dns.Server{Addr: address, Net: network, Handler: handler, NotifyStartedFunc: func() {
		close(notifyDone)
	}}

	go func() {
		defer GinkgoRecover()
		err := server.ListenAndServe()

		// NOTE because of this issue https://github.com/miekg/dns/issues/457
		//   we will experience a non-graceful shutdown when terminating the
		//   TCP server. We have to allow that particular error.
		Expect(err).To(Or(Not(HaveOccurred()), MatchError(ContainSubstring("use of closed network connection"))))
	}()

	Eventually(notifyDone).Should(BeClosed())

	return server
}

var _ = Describe("Upcheck", func() {
	var subject server.Upcheck
	var dnsHandler dns.Handler
	var udpServer *dns.Server
	var tcpServer *dns.Server
	var listenDomain string
	var ports map[string]int
	var addresses map[string]string
	upcheckDomain := "upcheck.bosh-dns."

	JustBeforeEach(func() {
		var err error
		ports["udp"], err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		ports["tcp"], err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
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
		dnsHandler = handlers.NewUpcheckHandler(&boshlogf.FakeLogger{})
	})

	Context("when the upcheck target is a malformed address", func() {
		DescribeTable("returns an error", func(network string) {
			subject = server.NewDNSAnswerValidatingUpcheck("~~~~~~~~~~", upcheckDomain, network)

			err := subject.IsUp()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("on %s: ", network))
			Expect(err.Error()).To(ContainSubstring("missing port in address"))
			Expect(err.Error()).To(ContainSubstring("~~~~~~~~~~"))
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the target server resolves the upcheck domain", func() {
		Context("when the target address is 127.0.0.1", func() {
			DescribeTable("it checks on 127.0.0.1", func(network string) {
				subject = server.NewDNSAnswerValidatingUpcheck(fmt.Sprintf("127.0.0.1:%d", ports[network]), upcheckDomain, network)

				err := subject.IsUp()
				Expect(err).NotTo(HaveOccurred())
			},
				Entry("when networking is udp", "udp"),
				Entry("when networking is tcp", "tcp"),
			)
		})

		Context("when the target address is 0.0.0.0", func() {
			DescribeTable("it checks on 127.0.0.1", func(network string) {
				subject = server.NewDNSAnswerValidatingUpcheck(fmt.Sprintf("0.0.0.0:%d", ports[network]), upcheckDomain, network)

				err := subject.IsUp()
				Expect(err).NotTo(HaveOccurred())
			},
				Entry("when networking is udp", "udp"),
				Entry("when networking is tcp", "tcp"),
			)
		})
	})

	Context("when the upcheck takes a long time", func() {
		DescribeTable("times out with error", func(network string) {
			// 203.0.113.0/24 is reserved for documentation as per RFC 5737
			subject = server.NewDNSAnswerValidatingUpcheck("203.0.113.1:30", upcheckDomain, network)

			err := subject.IsUp()
			Expect(err).To(HaveOccurred())
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the upcheck domain resolves with no answers", func() {
		BeforeEach(func() {
			dnsHandler = dns.HandlerFunc(func(r dns.ResponseWriter, m *dns.Msg) {
				m.Rcode = dns.RcodeSuccess
				r.WriteMsg(m)
			})
		})

		DescribeTable("returns with error", func(network string) {
			subject = server.NewDNSAnswerValidatingUpcheck(addresses[network], upcheckDomain, network)

			err := subject.IsUp()
			Expect(err).To(HaveOccurred())
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})

	Context("when the upcheck domain resolve failed", func() {
		BeforeEach(func() {
			dnsHandler = dns.HandlerFunc(func(r dns.ResponseWriter, m *dns.Msg) {
				m.Rcode = dns.RcodeServerFailure
				r.WriteMsg(m)
			})
		})

		DescribeTable("returns with error", func(network string) {
			subject = server.NewDNSAnswerValidatingUpcheck(addresses[network], upcheckDomain, network)

			err := subject.IsUp()
			Expect(err).To(HaveOccurred())
		},
			Entry("when networking is udp", "udp"),
			Entry("when networking is tcp", "tcp"),
		)
	})
})

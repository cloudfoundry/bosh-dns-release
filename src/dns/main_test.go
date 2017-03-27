package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"os/exec"
	"time"

	"net"

	"github.com/miekg/dns"
)

var _ = Describe("main", func() {
	var (
		cmd *exec.Cmd
	)

	BeforeEach(func() {
		cmd = exec.Command(pathToServer)
	})

	AfterEach(func() {
		cmd.Process.Kill()
	})

	DescribeTable("it responds to DNS requests",
		func(protocol string) {
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				session.Kill()
				session.Wait()
			}()

			time.Sleep(1 * time.Second)
			c := new(dns.Client)
			c.Net = protocol

			m := new(dns.Msg)
			zone := "example.com"

			m.SetQuestion(dns.Fqdn(zone), dns.TypeANY)
			r, _, err := c.Exchange(m, "127.0.0.1:9955")

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
		},
		Entry("when the request is udp", "udp"),
		Entry("when the request is tcp", "tcp"),
	)

	It("can respond to UDP messages up to 65535 bytes", func() {
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			session.Kill()
			session.Wait()
		}()

		time.Sleep(1 * time.Second)
		c := new(dns.Client)
		c.Net = "udp"

		m := new(dns.Msg)
		zone := "example.com"

		m.SetQuestion(dns.Fqdn(zone), dns.TypeANY)

		// 1800 is a semi magic number which we've determined will cause a truncation if the UDPSize is not set to 65535
		for i := 0; i < 1800; i++ {
			m.Question = append(m.Question, dns.Question{".", dns.TypeANY, dns.ClassINET})
		}

		r, _, err := c.Exchange(m, "127.0.0.1:9955")

		Expect(err).NotTo(HaveOccurred())
		Expect(r.Rcode).To(Equal(dns.RcodeSuccess))

	})

	It("exits 1 when fails to bind to the tcp port", func() {
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9955})
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer cmd.Process.Kill()

		session.Wait()
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("exits 1 when fails to bind to the udp port", func() {
		listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9955})
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer cmd.Process.Kill()

		session.Wait()
		Expect(session.ExitCode()).To(Equal(1))
	})
})

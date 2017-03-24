package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"os/exec"
	"time"

	"fmt"
	"github.com/miekg/dns"
	"net"
)

var _ = Describe("main", func() {
	var (
		cmd *exec.Cmd
	)

	BeforeEach(func() {
		cmd = exec.Command(pathToServer)
	})

	AfterEach(func() {
		fmt.Println("in the after each")
		cmd.Process.Kill()
	})

	It("responds to tcp messages", func() {
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			session.Kill()
			session.Wait()
		}()

		time.Sleep(1 * time.Second)
		c := new(dns.Client)
		c.Net = "tcp"

		m := new(dns.Msg)
		zone := "example.com"

		m.SetQuestion(dns.Fqdn(zone), dns.TypeANY)
		r, _, err := c.Exchange(m, "127.0.0.1:9955")

		Expect(err).NotTo(HaveOccurred())
		Expect(r.Rcode).To(Equal(dns.RcodeSuccess))

	})

	It("exits 1 when fails to bind to the port", func() {
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9955})
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer cmd.Process.Kill()

		session.Wait()
		Expect(session.ExitCode()).To(Equal(1))
	})
})

package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"os/exec"
	"time"

	"net"

	"io/ioutil"
	"strconv"

	"github.com/miekg/dns"
	"github.com/onsi/gomega/gbytes"
)

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(port)
}

var _ = Describe("main", func() {
	var (
		cmd           *exec.Cmd
		listenAddress string
		listenPort    int
	)

	BeforeEach(func() {
		configFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		listenAddress = "127.0.0.1"
		listenPort, err = getFreePort()
		Expect(err).NotTo(HaveOccurred())

		_, err = configFile.Write([]byte(fmt.Sprintf(`{
		  "address": "%s",
		  "port": %d
		}`, listenAddress, listenPort)))

		Expect(err).NotTo(HaveOccurred())

		args := []string{
			"--config",
			configFile.Name(),
		}

		cmd = exec.Command(pathToServer, args...)
	})

	AfterEach(func() {
		cmd.Process.Kill()
	})

	Describe("flags", func() {
		It("exits 1 if the config file has not been provided", func() {
			cmd = exec.Command(pathToServer)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("--config is a required flag"))
		})

		It("exits 1 if the config file does not exist", func() {
			args := []string{
				"--config",
				"some/fake/path",
			}

			cmd = exec.Command(pathToServer, args...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			session.Wait()

			Expect(session.ExitCode()).To(Equal(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("some/fake/path: no such file or directory"))
		})

		It("exits 1 if the config file is busted", func() {
			configFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = configFile.Write([]byte(fmt.Sprintf(`%%%`)))
			Expect(err).NotTo(HaveOccurred())

			args := []string{
				"--config",
				configFile.Name(),
			}

			cmd = exec.Command(pathToServer, args...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("invalid character '%' looking for beginning of value"))
		})
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
			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

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
		r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

		Expect(err).NotTo(HaveOccurred())
		Expect(r.Rcode).To(Equal(dns.RcodeSuccess))

	})

	It("exits 1 when fails to bind to the tcp port", func() {
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer cmd.Process.Kill()

		Eventually(session).Should(gexec.Exit(1))
	})

	It("exits 1 when fails to bind to the udp port", func() {
		listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer cmd.Process.Kill()

		Eventually(session).Should(gexec.Exit(1))
	})

	It("exits 1 and logs a helpful error message when the server times out binding to ports", func() {
		configFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		_, err = configFile.Write([]byte(fmt.Sprintf(`{
		  "address": "%s",
		  "port": %d,
		  "timeout": "0s"
		}`, listenAddress, listenPort)))

		Expect(err).NotTo(HaveOccurred())

		args := []string{
			"--config",
			configFile.Name(),
		}

		cmd = exec.Command(pathToServer, args...)

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session).Should(gexec.Exit(1))
		Eventually(session.Out).Should(gbytes.Say("timed out waiting for server to bind"))
	})
})

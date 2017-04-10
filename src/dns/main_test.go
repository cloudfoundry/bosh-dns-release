package main_test

import (
	"errors"
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

	"syscall"

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
		listenAddress string
		listenPort    int
	)

	BeforeEach(func() {
		var err error

		listenAddress = "127.0.0.1"
		listenPort, err = getFreePort()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("flags", func() {
		It("exits 1 if the config file has not been provided", func() {
			cmd := exec.Command(pathToServer)
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

			cmd := exec.Command(pathToServer, args...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			session.Wait()

			Expect(session.ExitCode()).To(Equal(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("some/fake/path: no such file or directory"))
		})

		It("exits 1 if the config file is busted", func() {
			cmd := newCommandWithConfig("%%%")

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("invalid character '%' looking for beginning of value"))
		})
	})

	Context("when the server starts successfully", func() {
		var (
			cmd     *exec.Cmd
			session *gexec.Session
		)

		BeforeEach(func() {
			var err error

			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": %q,
				"port": %d
			}`, listenAddress, listenPort))

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(waitForServer(listenPort)).To(Succeed())
		})

		AfterEach(func() {
			if cmd.Process != nil {
				session.Kill()
				session.Wait()
			}
		})

		DescribeTable("it responds to DNS requests",
			func(protocol string) {
				c := &dns.Client{
					Net: protocol,
				}

				m := &dns.Msg{}

				m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)
				r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

				Expect(err).NotTo(HaveOccurred())
				Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
			},
			Entry("when the request is udp", "udp"),
			Entry("when the request is tcp", "tcp"),
		)

		Context("handlers", func() {
			Context("healthcheck.bosh-dns.", func() {
				It("responds with a success rcode", func() {
					c := &dns.Client{
						Net: "tcp",
					}

					m := &dns.Msg{}

					m.SetQuestion("healthcheck.bosh-dns.", dns.TypeA)
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
				})
			})
		})

		It("can respond to UDP messages up to 65535 bytes", func() {
			c := &dns.Client{
				Net: "udp",
			}

			m := &dns.Msg{}

			m.SetQuestion("healthcheck.bosh-dns.", dns.TypeANY)

			// 353 is a semi magic number which we've determined will cause a truncation if the UDPSize is not set to 65535
			for i := 0; i < 353; i++ {
				m.Question = append(m.Question, dns.Question{"healthcheck.bosh-dns.", dns.TypeANY, dns.ClassINET})
			}

			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("gracefully shuts down on TERM", func() {
			session.Signal(syscall.SIGTERM)

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when recursing has been enabled", func() {
		It("will timeout after the recursor_timeout has been reached", func() {
			l, err := net.Listen("tcp", ":0")
			Expect(err).NotTo(HaveOccurred())
			defer l.Close()

			go func() {
				l.Accept()
			}()

			cmd := newCommandWithConfig(fmt.Sprintf(`{
				"address": %q,
				"port": %d,
				"recursors": [%q],
				"recursor_timeout": %q
			}`, listenAddress, listenPort, l.Addr().String(), "1s"))

			_, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(waitForServer(listenPort)).To(Succeed())

			timeoutNeverToBeReached := 10 * time.Second
			c := &dns.Client{
				Net:     "tcp",
				Timeout: timeoutNeverToBeReached,
			}

			m := &dns.Msg{}

			m.SetQuestion("bosh.io.", dns.TypeANY)

			startTime := time.Now()
			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
			Expect(time.Now().Sub(startTime)).Should(BeNumerically(">=", 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeServerFailure))
		})
	})

	Context("failure cases", func() {
		var (
			cmd *exec.Cmd
		)

		BeforeEach(func() {
			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": "%s",
				"port": %d,
				"recursors": ["8.8.8.8"]
			}`, listenAddress, listenPort))
		})

		It("exits 1 when fails to bind to the tcp port", func() {
			listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
			Expect(err).NotTo(HaveOccurred())
			defer listener.Close()

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
		})

		It("exits 1 when fails to bind to the udp port", func() {
			listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
			Expect(err).NotTo(HaveOccurred())
			defer listener.Close()

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
		})

		It("exits 1 and logs a helpful error message when the server times out binding to ports", func() {
			cmd := newCommandWithConfig(fmt.Sprintf(`{
				"address": "%s",
				"port": %d,
				"timeout": "0s"
			}`, listenAddress, listenPort))

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("timed out waiting for server to bind"))
		})
	})
})

func newCommandWithConfig(config string) *exec.Cmd {
	configFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(config))

	Expect(err).NotTo(HaveOccurred())

	args := []string{
		"--config",
		configFile.Name(),
	}

	return exec.Command(pathToServer, args...)
}

func waitForServer(port int) error {
	for i := 0; i < 20; i++ {
		c, err := net.Dial("tcp", fmt.Sprintf(":%s", strconv.Itoa(port)))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		c.Close()
		return nil
	}

	return errors.New("dns server failed to start")
}

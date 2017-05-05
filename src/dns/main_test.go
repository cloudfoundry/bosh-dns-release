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

	"runtime"
	"syscall"

	"github.com/miekg/dns"
	"github.com/onsi/gomega/gbytes"
	"os"
	"path"
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
			Expect(session.Err).To(gbytes.Say("[main].*ERROR - --config is a required flag"))
		})

		It("exits 1 if the config file does not exist", func() {
			args := []string{
				"--config",
				"some/fake/path",
			}

			cmd := exec.Command(pathToServer, args...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("[main].*ERROR - Unable to find config file at 'some/fake/path'"))
		})

		It("exits 1 if the config file is busted", func() {
			cmd := newCommandWithConfig("{")

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("[main].*ERROR - unexpected end of JSON input"))
		})
	})

	Context("when the server starts successfully", func() {
		var (
			cmd        *exec.Cmd
			session    *gexec.Session
			aliasesDir string
		)

		BeforeEach(func() {
			var err error

			recordsFile, err := ioutil.TempFile("", "recordsjson")
			Expect(err).NotTo(HaveOccurred())

			_, err = recordsFile.Write([]byte(fmt.Sprint(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
				"record_infos": [
					["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"],
					["my-instance-2", "my-group", "az1", "my-network", "my-deployment-2", "123.456.789.000"]
				]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			aliasesDir, err = ioutil.TempDir("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			aliasesFile1, err := ioutil.TempFile(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close()
			_, err = aliasesFile1.Write([]byte(fmt.Sprint(`{
				"hc.alias.": ["healthcheck.bosh-dns."]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := ioutil.TempFile(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close()
			_, err = aliasesFile2.Write([]byte(fmt.Sprint(`{
				"internal.alias.": ["my-instance-2.my-group.my-network.my-deployment-2.bosh.","my-instance.my-group.my-network.my-deployment.bosh."]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": %q,
				"port": %d,
				"records_file": %q,
				"alias_files_glob": %q
			}`, listenAddress, listenPort, recordsFile.Name(), path.Join(aliasesDir, "*")))

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(waitForServer(listenPort)).To(Succeed())
		})

		AfterEach(func() {
			if cmd.Process != nil {
				session.Kill()
				session.Wait()
			}

			Expect(os.RemoveAll(aliasesDir)).To(Succeed())
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
			var (
				c *dns.Client
				m *dns.Msg
			)

			BeforeEach(func() {
				c = &dns.Client{}
				m = &dns.Msg{}
			})

			Describe("alias resolution", func() {
				Context("with only one resolving address", func() {
					BeforeEach(func() {
						m.SetQuestion("hc.alias.", dns.TypeA)
					})

					It("resolves to the appropriate domain before deferring to mux", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(Equal([]dns.RR{}))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.HealthCheckHandler Request \[1\] \[healthcheck\.bosh-dns\.\] 0 \d+ns`))
					})
				})

				Context("with multiple resolving addresses", func() {
					BeforeEach(func() {
						m.Question = []dns.Question{
							{Name: "internal.alias.", Qtype: dns.TypeA},
						}
					})

					It("resolves all domains before deferring to mux", func() {
						_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.DiscoveryHandler Request \[1,1\] \[my-instance-2\.my-group\.my-network\.my-deployment-2\.bosh\.,my-instance\.my-group\.my-network\.my-deployment\.bosh\.\] 0 \d+ns`))
					})
				})
			})

			Context("healthcheck.bosh-dns.", func() {
				BeforeEach(func() {
					m.SetQuestion("healthcheck.bosh-dns.", dns.TypeA)
				})

				It("responds with a success rcode", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
				})

				It("logs handler time", func() {
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.HealthCheckHandler Request \[1\] \[healthcheck\.bosh-dns\.\] 0 \d+ns`))
				})
			})

			Context("arpa.", func() {
				BeforeEach(func() {
					m.SetQuestion("109.22.25.104.in-addr.arpa.", dns.TypePTR)
				})

				It("responds to arpa. requests with an rcode server failure", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeServerFailure))
					Expect(r.Authoritative).To(BeTrue())
					Expect(r.RecursionAvailable).To(BeFalse())
				})

				It("logs handler time", func() {
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ArpaHandler Request \[12\] \[109\.22\.25\.104\.in-addr\.arpa\.\] 2 \d+ns`))
				})
			})

			Context("bosh.", func() {
				BeforeEach(func() {
					m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
				})

				It("responds to A queries for bosh. with content from the record API", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Expect(r.Answer).To(HaveLen(1))

					answer := r.Answer[0]
					header := answer.Header()

					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Authoritative).To(BeTrue())
					Expect(r.RecursionAvailable).To(BeFalse())

					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))
				})

				It("logs handler time", func() {
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.DiscoveryHandler Request \[1\] \[my-instance\.my-group\.my-network\.my-deployment\.bosh\.\] 0 \d+ns`))
				})
			})
		})

		It("gracefully shuts down on TERM", func() {
			if runtime.GOOS == "windows" {
				Skip("TERM is not supported in Windows")
			}

			session.Signal(syscall.SIGTERM)

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when recursing has been enabled", func() {
		var (
			cmd     *exec.Cmd
			session *gexec.Session
		)

		It("will timeout after the recursor_timeout has been reached", func() {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())
			defer l.Close()

			go func() {
				l.Accept()
			}()

			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": %q,
				"port": %d,
				"recursors": [%q],
				"recursor_timeout": %q
			}`, listenAddress, listenPort, l.Addr().String(), "1s"))

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
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

			Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ForwardHandler Request \[255\] \[bosh\.io\.\] 2 \d+ns`))
		})

		AfterEach(func() {
			if cmd.Process != nil {
				session.Kill()
				session.Wait()
			}
		})
	})

	Context("failure cases", func() {
		var (
			cmd        *exec.Cmd
			aliasesDir string
		)

		BeforeEach(func() {
			var err error
			aliasesDir, err = ioutil.TempDir("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": "%s",
				"port": %d,
				"recursors": ["8.8.8.8"],
				"alias_files_glob": %q
			}`, listenAddress, listenPort, path.Join(aliasesDir, "*")))
		})

		AfterEach(func() {
			Expect(os.RemoveAll(aliasesDir)).To(Succeed())
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
			Eventually(session.Err).Should(gbytes.Say("[main].*ERROR - timed out waiting for server to bind"))
		})

		It("exits 1 and logs a message when the globbed config files contain a broken alias config", func() {
			aliasesFile1, err := ioutil.TempFile(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close()
			_, err = aliasesFile1.Write([]byte(fmt.Sprint(`{
				"hc.alias.": ["healthcheck.bosh-dns."]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := ioutil.TempFile(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close()
			_, err = aliasesFile2.Write([]byte(`{"malformed":"aliasfile"}`))
			Expect(err).NotTo(HaveOccurred())

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say(`[main].*ERROR - loading alias configuration:.*alias config file malformed:`))
			Expect(session.Err.Contents()).To(ContainSubstring(fmt.Sprintf(`alias config file malformed: %s`, aliasesFile2.Name())))
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
		c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", strconv.Itoa(port)))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		c.Close()
		return nil
	}

	return errors.New("dns server failed to start")
}

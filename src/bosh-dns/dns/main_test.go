package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"

	"os/exec"
	"time"

	"net"

	"io/ioutil"

	"runtime"
	"syscall"

	"os"
	"path"

	"crypto/tls"
	"crypto/x509"
	"net/http"

	"github.com/miekg/dns"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("main", func() {
	var (
		listenAddress string
		listenPort    int
	)

	BeforeEach(func() {
		listenAddress = "127.0.0.1"
		var err error
		listenPort, err = getFreePort()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("flags", func() {
		It("exits 1 if the config file has not been provided", func() {
			cmd := exec.Command(pathToServer)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Out).To(gbytes.Say("[main].*ERROR - --config is a required flag"))
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
			Expect(session.Out).To(gbytes.Say("[main].*ERROR - Unable to find config file at 'some/fake/path'"))
		})

		It("exits 1 if the config file is busted", func() {
			cmd := newCommandWithConfig("{")

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Out).To(gbytes.Say("[main].*ERROR - unexpected end of JSON input"))
		})
	})

	Context("when the server starts successfully", func() {
		var (
			cmd                    *exec.Cmd
			session                *gexec.Session
			aliasesDir             string
			recordsFilePath        string
			checkInterval          string
			httpJSONServer         *ghttp.Server
			httpJSONCachingEnabled bool
		)

		BeforeEach(func() {
			checkInterval = "100ms"
			httpJSONCachingEnabled = false
		})

		JustBeforeEach(func() {
			var err error

			recordsFile, err := ioutil.TempFile("", "recordsjson")
			Expect(err).NotTo(HaveOccurred())

			_, err = recordsFile.Write([]byte(fmt.Sprint(`{
				"record_keys": ["id", "instance_group", "group_ids", "az", "az_id","network", "deployment", "ip", "domain"],
				"record_infos": [
					["my-instance", "my-group", ["7"], "az1", "1", "my-network", "my-deployment", "127.0.0.1", "bosh"],
					["my-instance-1", "my-group", ["7"], "az2", "2", "my-network", "my-deployment", "127.0.0.2", "bosh"],
					["my-instance-2", "my-group", ["8"], "az2", "2", "my-network", "my-deployment-2", "127.0.0.3", "bosh"],
					["my-instance-1", "my-group", ["7"], "az1", "1", "my-network", "my-deployment", "127.0.0.2", "foo"],
					["my-instance-2", "my-group", ["8"], "az2", "2", "my-network", "my-deployment-2", "127.0.0.3", "foo"],
					["primer-instance", "primer-group", ["9"], "az1", "1", "primer-network", "primer-deployment", "127.0.0.254", "primer"]
				]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			recordsFilePath = recordsFile.Name()

			aliasesDir, err = ioutil.TempDir("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			aliasesFile1, err := ioutil.TempFile(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close()
			_, err = aliasesFile1.Write([]byte(fmt.Sprint(`{
				"uc.alias.": ["upcheck.bosh-dns."]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := ioutil.TempFile(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close()
			_, err = aliasesFile2.Write([]byte(fmt.Sprint(`{
				"one.alias.": ["my-instance.my-group.my-network.my-deployment.bosh."],
				"internal.alias.": ["my-instance-2.my-group.my-network.my-deployment-2.bosh.","my-instance.my-group.my-network.my-deployment.bosh."],
				"group.internal.alias.": ["*.my-group.my-network.my-deployment.bosh."],
				"ip.alias.": ["10.11.12.13"]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			httpJSONServer = ghttp.NewUnstartedServer()
			httpJSONServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/", "name=app-id.internal-domain.&type=255"),
				ghttp.RespondWith(http.StatusOK, `{
  "Status": 0,
  "TC": false,
  "RD": true,
  "RA": true,
  "AD": false,
  "CD": false,
  "Question":
  [
    {
      "name": "app-id.internal-domain.",
      "type": 28
    }
  ],
  "Answer":
  [
    {
      "name": "app-id.internal-domain.",
      "type": 1,
      "TTL": 1526,
      "data": "192.168.0.1"
    }
  ],
  "Additional": [ ],
  "edns_client_subnet": "12.34.56.78/0"
}`),
			),
			)
			httpJSONServer.HTTPTestServer.Start()
			configContents, err := json.Marshal(map[string]interface{}{
				"address":          listenAddress,
				"port":             listenPort,
				"records_file":     recordsFilePath,
				"alias_files_glob": path.Join(aliasesDir, "*"),
				"upcheck_domains":  []string{"health.check.bosh.", "health.check.ca."},
				"health": map[string]interface{}{
					"enabled":          true,
					"port":             2345 + config.GinkgoConfig.ParallelNode,
					"ca_file":          "../healthcheck/assets/test_certs/test_ca.pem",
					"certificate_file": "../healthcheck/assets/test_certs/test_client.pem",
					"private_key_file": "../healthcheck/assets/test_certs/test_client.key",
					"check_interval":   checkInterval,
				},
				"handlers": []map[string]interface{}{{
					"domain": "internal-domain.",
					"cache": map[string]interface{}{
						"enabled": httpJSONCachingEnabled,
					},
					"source": map[string]interface{}{
						"type": "http",
						"url":  httpJSONServer.URL(),
					},
				}},
			})
			Expect(err).NotTo(HaveOccurred())
			cmd = newCommandWithConfig(string(configContents))

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(waitForServer(listenPort)).To(Succeed())

			Eventually(func() int {
				c := &dns.Client{}
				m := &dns.Msg{}
				m.SetQuestion("primer-instance.primer-group.primer-network.primer-deployment.primer.", dns.TypeANY)
				r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
				if err != nil {
					return -1
				}

				return r.Rcode
			}, 5*time.Second).Should(Equal(dns.RcodeSuccess))
		})

		AfterEach(func() {
			if cmd.Process != nil {
				session.Kill()
				session.Wait()
			}

			Expect(os.RemoveAll(aliasesDir)).To(Succeed())

			httpJSONServer.Close()
		})

		DescribeTable("it responds to DNS requests",
			func(protocol string) {
				c := &dns.Client{
					Net: protocol,
				}

				m := &dns.Msg{}

				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
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
						m.SetQuestion("one.alias.", dns.TypeA)
					})

					It("resolves to the appropriate domain before deferring to mux", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal("one.alias."))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[one\.alias\.\] 0 \d+ns`))
					})
				})

				Context("with an address resolving to an IP", func() {
					BeforeEach(func() {
						m.SetQuestion("ip.alias.", dns.TypeA)
					})

					It("resolves to the appropriate domain before deferring to mux", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal("ip.alias."))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("10.11.12.13"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[ip\.alias\.\] 0 \d+ns`))
					})
				})

				Context("with multiple resolving addresses", func() {
					It("resolves all domains before deferring to mux", func() {
						m.Question = []dns.Question{{Name: "internal.alias.", Qtype: dns.TypeA}}
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(2))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal("internal.alias."))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))

						Expect(response.Answer[1].Header().Name).To(Equal("internal.alias."))
						Expect(response.Answer[1].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[1].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[1].Header().Ttl).To(Equal(uint32(0)))

						ips := []string{response.Answer[0].(*dns.A).A.String(), response.Answer[1].(*dns.A).A.String()}
						Expect(ips).To(ConsistOf("127.0.0.1", "127.0.0.3"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[internal\.alias\.\] 0 \d+ns`))
					})

					Context("with a group alias", func() {
						It("returns all records belonging to the correct group", func() {
							m.Question = []dns.Question{{Name: "group.internal.alias.", Qtype: dns.TypeA}}

							response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
							Expect(err).NotTo(HaveOccurred())

							Expect(response.Answer).To(HaveLen(2))
							Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(response.Answer[0].Header().Name).To(Equal("group.internal.alias."))
							Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
							Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
							Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))

							Expect(response.Answer[1].Header().Name).To(Equal("group.internal.alias."))
							Expect(response.Answer[1].Header().Rrtype).To(Equal(dns.TypeA))
							Expect(response.Answer[1].Header().Class).To(Equal(uint16(dns.ClassINET)))
							Expect(response.Answer[1].Header().Ttl).To(Equal(uint32(0)))

							ips := []string{response.Answer[0].(*dns.A).A.String(), response.Answer[1].(*dns.A).A.String()}
							Expect(ips).To(ConsistOf("127.0.0.1", "127.0.0.2"))

							Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[group\.internal\.alias\.\] 0 \d+ns`))
						})
					})
				})
			})

			Context("upcheck domains", func() {
				BeforeEach(func() {
					m.SetQuestion("health.check.bosh.", dns.TypeA)
				})

				It("responds with a success rcode", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal("health.check.bosh."))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

					m.SetQuestion("health.check.ca.", dns.TypeA)
					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal("health.check.ca."))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))
				})

				It("logs handler time", func() {
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.UpcheckHandler Request \[1\] \[health\.check\.bosh\.\] 0 \d+ns`))
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

			Context("domains from records.json", func() {
				It("can interpret AZ-specific queries", func() {
					m.SetQuestion("q-a1s0.my-group.my-network.my-deployment.bosh.", dns.TypeA)

					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Expect(r.Answer).To(HaveLen(1))

					answer := r.Answer[0]
					header := answer.Header()

					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Authoritative).To(BeTrue())
					Expect(r.RecursionAvailable).To(BeTrue())

					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					Expect(answer.(*dns.A).A.String()).To(Equal("127.0.0.1"))

					m.SetQuestion("q-a2s0.my-group.my-network.my-deployment.bosh.", dns.TypeA)

					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Expect(r.Answer).To(HaveLen(1))

					answer = r.Answer[0]
					header = answer.Header()

					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Authoritative).To(BeTrue())
					Expect(r.RecursionAvailable).To(BeTrue())

					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					Expect(answer.(*dns.A).A.String()).To(Equal("127.0.0.2"))
				})

				It("can interpret abbreviated group encoding", func() {
					By("understanding q- queries", func() {
						m.SetQuestion("q-a1s0.q-g7.foo.", dns.TypeA)

						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(r.Answer).To(HaveLen(1))

						answer := r.Answer[0]
						header := answer.Header()

						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Authoritative).To(BeTrue())
						Expect(r.RecursionAvailable).To(BeTrue())

						Expect(header.Rrtype).To(Equal(dns.TypeA))
						Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
						Expect(header.Ttl).To(Equal(uint32(0)))

						Expect(answer.(*dns.A).A.String()).To(Equal("127.0.0.2"))
					})

					By("understanding specific instance hosts", func() {
						m.SetQuestion("my-instance-1.q-g7.foo.", dns.TypeA)

						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(r.Answer).To(HaveLen(1))

						answer := r.Answer[0]
						header := answer.Header()

						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Authoritative).To(BeTrue())
						Expect(r.RecursionAvailable).To(BeTrue())

						Expect(header.Rrtype).To(Equal(dns.TypeA))
						Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
						Expect(header.Ttl).To(Equal(uint32(0)))

						Expect(answer.(*dns.A).A.String()).To(Equal("127.0.0.2"))
					})
				})

				It("responds to A queries for foo. with content from the record API", func() {
					m.SetQuestion("my-instance-1.my-group.my-network.my-deployment.foo.", dns.TypeA)

					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Expect(r.Answer).To(HaveLen(1))

					answer := r.Answer[0]
					header := answer.Header()

					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Authoritative).To(BeTrue())
					Expect(r.RecursionAvailable).To(BeTrue())

					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					Expect(answer.(*dns.A).A.String()).To(Equal("127.0.0.2"))
				})

				It("logs handler time", func() {
					m.SetQuestion("my-instance-1.my-group.my-network.my-deployment.foo.", dns.TypeA)

					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.DiscoveryHandler Request \[1\] \[my-instance-1\.my-group\.my-network\.my-deployment\.foo\.\] 0 \d+ns`))
				})
			})

			Context("changing records.json", func() {
				JustBeforeEach(func() {
					var err error
					err = ioutil.WriteFile(recordsFilePath, []byte(fmt.Sprint(`{
						"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
						"record_infos": [
							["my-instance", "my-group", "az1", "my-network", "my-deployment", "127.0.0.3", "bosh"]
						]
					}`)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("picks up the changes", func() {
					Eventually(func() string {
						m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(r.Answer).To(HaveLen(1))

						answer := r.Answer[0]
						Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))

						return answer.(*dns.A).A.String()
					}).Should(Equal("127.0.0.3"))
				})
			})

			Context("http json domains", func() {
				It("serves the addresses from the http server", func() {
					c := &dns.Client{Net: "tcp"}

					m := &dns.Msg{}

					m.SetQuestion("app-id.internal-domain.", dns.TypeANY)
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))

					answer0 := r.Answer[0].(*dns.A)
					Expect(answer0.A.String()).To(Equal("192.168.0.1"))
				})

				Context("when caching is enabled", func() {
					BeforeEach(func() {
						httpJSONCachingEnabled = true
					})

					It("should return cached answers", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						m.SetQuestion("app-id.internal-domain.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.168.0.1"))

						m = &dns.Msg{}

						m.SetQuestion("app-id.internal-domain.", dns.TypeANY)
						r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 = r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.168.0.1"))
					})
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

		Context("health checking", func() {
			var healthServers []*ghttp.Server

			Context("when both servers are running and return responses", func() {
				BeforeEach(func() {
					healthServers = []*ghttp.Server{
						newFakeHealthServer("127.0.0.1", "running"),
						// sudo ifconfig lo0 alias 127.0.0.2 up # on osx
						newFakeHealthServer("127.0.0.2", "stopped"),
					}
				})

				AfterEach(func() {
					for _, server := range healthServers {
						server.Close()
					}
				})

				It("checks healthiness of results", func() {
					c := &dns.Client{Net: "udp"}

					m := &dns.Msg{}
					m.SetQuestion("q-s0.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(2))

					ips := []string{r.Answer[0].(*dns.A).A.String(), r.Answer[1].(*dns.A).A.String()}
					Expect(ips).To(ConsistOf("127.0.0.1", "127.0.0.2"))

					serverRequestLen := func(server *ghttp.Server) func() int {
						return func() int {
							return len(server.ReceivedRequests())
						}
					}
					Eventually(serverRequestLen(healthServers[0]), 5*time.Second).Should(BeNumerically(">", 2))
					Eventually(serverRequestLen(healthServers[1]), 5*time.Second).Should(BeNumerically(">", 2))

					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))

					ips = []string{r.Answer[0].(*dns.A).A.String()}
					Expect(ips).To(ConsistOf("127.0.0.1"))

					err = ioutil.WriteFile(recordsFilePath, []byte(fmt.Sprint(`{
						"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
						"record_infos": [
							["my-instance", "my-group", "az1", "my-network", "my-deployment", "127.0.0.1", "bosh"]
						]
					}`)), 0644)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() bool {
						startLength := len(healthServers[1].ReceivedRequests())
						time.Sleep(200 * time.Millisecond)
						finalLength := len(healthServers[1].ReceivedRequests())
						return startLength == finalLength
					}).Should(BeTrue())
				})
			})

			Context("when a server is not returning responses", func() {
				var brokenServer *ghttp.Server

				BeforeEach(func() {
					checkInterval = "1m"
					brokenServer = newFakeHealthServer("127.0.0.2", "stopped")
					brokenServer.RouteToHandler("GET", "/health", ghttp.RespondWith(http.StatusGatewayTimeout, ``))

					healthServers = []*ghttp.Server{
						newFakeHealthServer("127.0.0.1", "running"),
						// sudo ifconfig lo0 alias 127.0.0.2 up # on osx
						brokenServer,
					}
				})

				AfterEach(func() {
					for _, server := range healthServers {
						server.Close()
					}
				})

				It("retries errors", func() {
					c := &dns.Client{Net: "udp"}

					m := &dns.Msg{}
					m.SetQuestion("q-s0.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())

					serverRequestLen := func(server *ghttp.Server) func() int {
						return func() int {
							return len(server.ReceivedRequests())
						}
					}
					Eventually(serverRequestLen(healthServers[0]), 4*time.Second).Should(BeNumerically("==", 1))
					Eventually(serverRequestLen(healthServers[1]), 4*time.Second).Should(BeNumerically("==", 4))
				})
			})
		})
	})

	Context("when specific recursors have been configured", func() {
		var (
			cmd     *exec.Cmd
			session *gexec.Session
		)

		It("will timeout after the recursor_timeout has been reached", func() {
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 9000+config.GinkgoConfig.ParallelNode))
			Expect(err).NotTo(HaveOccurred())
			defer l.Close()

			go func() {
				defer GinkgoRecover()
				_, err := l.Accept()
				Expect(err).NotTo(HaveOccurred())
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
			Expect(time.Now().Sub(startTime)).Should(BeNumerically(">", 999*time.Millisecond))
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeServerFailure))

			Eventually(session.Out).Should(gbytes.Say(`\[ForwardHandler\].*handlers\.ForwardHandler Request \[255\] \[bosh\.io\.\] 2 \[no response from recursors\] \d+ns`))
		})

		It("logs the recursor used to resolve", func() {
			var err error

			cmd = newCommandWithConfig(fmt.Sprintf(`{
				"address": %q,
				"port": %d,
				"recursors": [%q],
				"recursor_timeout": %q
			}`, listenAddress, listenPort, "8.8.8.8", "1s"))

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(waitForServer(listenPort)).To(Succeed())

			c := &dns.Client{}
			m := &dns.Msg{}
			m.SetQuestion("bosh.io.", dns.TypeANY)

			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeSuccess))

			Eventually(session.Out).Should(gbytes.Say(`\[ForwardHandler\].*handlers\.ForwardHandler Request \[255\] \[bosh\.io\.\] 0 \[recursor=8\.8\.8\.8:53\] \d+ns`))
			Consistently(session.Out).ShouldNot(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ForwardHandler Request \[255\] \[bosh\.io\.\] 0 \d+ns`))
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
				"upcheck_domains":["upcheck.bosh-dns."],
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
				"upcheck_domains": ["upcheck.bosh-dns."],
				"timeout": "0s"
			}`, listenAddress, listenPort))

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - timed out waiting for server to bind"))
		})

		It("exits 1 and logs a helpful error message when the config contains an unknown handler source type", func() {
			cmd := newCommandWithConfig(fmt.Sprintf(`{
				"address": "%s",
				"port": %d,
				"upcheck_domains": ["upcheck.bosh-dns."],
				"handlers": [
					{
						"domain": "internal.domain.",
						"source": {
							"type": "mistyped_dns"
						}
					}
				]
			}`, listenAddress, listenPort))

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - Unexpected handler source type: mistyped_dns"))
		})

		It("exits 1 and logs a message when the globbed config files contain a broken alias config", func() {
			aliasesFile1, err := ioutil.TempFile(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close()
			_, err = aliasesFile1.Write([]byte(fmt.Sprint(`{
				"uc.alias.": ["upcheck.bosh-dns."]
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
			Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - loading alias configuration:.*alias config file malformed:`))
			Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf(`alias config file malformed: %s`, aliasesFile2.Name())))
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
		c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		err = c.Close()
		Expect(err).NotTo(HaveOccurred())
		return nil
	}

	return errors.New("dns server failed to start")
}

func newFakeHealthServer(ip, state string) *ghttp.Server {
	caCert, err := ioutil.ReadFile("../healthcheck/assets/test_certs/test_ca.pem")
	Expect(err).ToNot(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair("../healthcheck/assets/test_certs/test_server.pem", "../healthcheck/assets/test_certs/test_server.key")
	Expect(err).ToNot(HaveOccurred())

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithIdentity(cert),
		tlsconfig.WithInternalServiceDefaults(),
	)

	serverConfig := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))
	serverConfig.BuildNameToCertificate()

	server := ghttp.NewUnstartedServer()
	err = server.HTTPTestServer.Listener.Close()
	Expect(err).NotTo(HaveOccurred())

	port := 2345 + config.GinkgoConfig.ParallelNode
	server.HTTPTestServer.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	Expect(err).ToNot(HaveOccurred(),
		fmt.Sprintf(`
===========================================
 ATTENTION: on macOS you may need to run
    sudo ifconfig lo0 alias %s up
===========================================
`, ip),
	)

	server.HTTPTestServer.TLS = serverConfig

	server.RouteToHandler("GET", "/health", ghttp.RespondWith(http.StatusOK, `{"state": "`+state+`"}`))
	server.HTTPTestServer.StartTLS()

	return server
}

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

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}

	return intPort, nil
}

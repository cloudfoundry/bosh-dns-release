package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"bosh-dns/dns/api"
	"bosh-dns/dns/config"
	handlersconfig "bosh-dns/dns/config/handlers"
	"bosh-dns/dns/internal/testhelpers"
	"bosh-dns/tlsclient"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("main", func() {
	var (
		listenAddress  string
		listenPort     int
		listenAddress2 string
		listenPort2    int
		listenAPIPort  int
		recursorList   []string
		jobsDir        string
	)

	BeforeEach(func() {
		listenAddress = "127.0.0.1"
		var err error
		listenPort, err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		listenAPIPort, err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())

		jobsDir, err = ioutil.TempDir("", "jobs")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(jobsDir)).To(Succeed())
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
			configFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = configFile.Write([]byte("{"))
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(pathToServer, "--config", configFile.Name())
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Out).To(gbytes.Say("[main].*ERROR - unexpected end of JSON input"))
		})
	})

	Context("when the server starts successfully", func() {
		var (
			addressesDir        string
			aliasesDir          string
			apiClient           *httpclient.HTTPClient
			checkInterval       time.Duration
			cmd                 *exec.Cmd
			handlersDir         string
			healthEnabled       bool
			httpJSONServer      *ghttp.Server
			recordsFilePath     string
			session             *gexec.Session
			recordsJSONContent  string
			aliases1JSONContent string
			aliases2JSONContent string
		)

		BeforeEach(func() {
			checkInterval = 100 * time.Millisecond
			recordsJSONContent = `{
				"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id","network", "deployment", "ip", "domain"],
				"record_infos": [
					["my-instance", "0", "my-group", ["7","1","2","3"], "az1", "1", "my-network", "my-deployment", "127.0.0.1", "bosh"],
					["my-instance-1", "1", "my-group", ["7"], "az2", "2", "my-network", "my-deployment", "127.0.0.2", "bosh"],
					["my-instance-2", "2", "my-group", ["8"], "az2", "2", "my-network", "my-deployment-2", "127.0.0.3", "bosh"],
					["my-instance-3", "3", "my-group", ["7"], "az1", "1", "my-network", "my-deployment", "127.0.0.2", "foo"],
					["my-instance-4", "4", "my-group", ["8"], "az2", "2", "my-network", "my-deployment-2", "127.0.0.3", "foo"],
					["primer-instance", "5", "primer-group", ["9"], "az1", "1", "primer-network", "primer-deployment", "127.0.0.254", "primer"]
				],
				"aliases": {
					"texas.nebraska": [{
						"group_id": "1",
						"root_domain": "bosh"
					}],
				  "_.placeholder.alias": [{
						"group_id": "7",
						"placeholder_type": "uuid",
						"health_filter": "all",
						"initial_health_check": "synchronous",
						"root_domain": "bosh"
					}]
				}
			}`
			aliases2JSONContent = `{
				"one.alias.": ["my-instance.my-group.my-network.my-deployment.bosh."],
				"internal.alias.": ["my-instance-2.my-group.my-network.my-deployment-2.bosh.","my-instance.my-group.my-network.my-deployment.bosh."],
				"group.internal.alias.": ["*.my-group.my-network.my-deployment.bosh."],
				"glob.internal.alias.": ["*.*y-group.my-network.my-deployment.bosh."],
				"anotherglob.internal.alias.": ["*.my-group*.my-network.my-deployment.bosh."],
				"yetanotherglob.internal.alias.": ["*.*.my-network.my-deployment.bosh."],
				"ip.alias.": ["10.11.12.13"]
			}`
			aliases1JSONContent = `{
				"uc.alias.": ["upcheck.bosh-dns."]
			}`

			healthEnabled = true
			recursorList = []string{}
		})

		JustBeforeEach(func() {
			var err error

			recordsFile, err := ioutil.TempFile("", "recordsjson")
			Expect(err).NotTo(HaveOccurred())

			_, err = recordsFile.Write([]byte(recordsJSONContent))
			Expect(err).NotTo(HaveOccurred())

			recordsFilePath = recordsFile.Name()

			addressesDir, err = ioutil.TempDir("", "addresses")
			Expect(err).NotTo(HaveOccurred())

			listenAddress2, err = localIP()
			Expect(err).NotTo(HaveOccurred())

			listenPort2, err = testhelpers.GetFreePort()
			Expect(err).NotTo(HaveOccurred())

			addressesFile, err := ioutil.TempFile(addressesDir, "addresses")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile.Close()
			_, err = addressesFile.Write([]byte(fmt.Sprintf(`[{"address": "%s", "port": %d }]`, listenAddress2, listenPort2)))
			Expect(err).NotTo(HaveOccurred())

			aliasesDir, err = ioutil.TempDir("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			aliasesFile1, err := ioutil.TempFile(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close()
			_, err = aliasesFile1.Write([]byte(aliases1JSONContent))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := ioutil.TempFile(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close()
			_, err = aliasesFile2.Write([]byte(aliases2JSONContent))
			Expect(err).NotTo(HaveOccurred())

			// httpJSONServer, handlersDir = setupHttpServer(handlerCachingEnabled, recursorPort)
			var handlersFilesGlob string
			if handlersDir != "" {
				handlersFilesGlob = path.Join(handlersDir, "*")
			}

			logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard)
			apiClient, err = tlsclient.NewFromFiles(
				"api.bosh-dns",
				"api/assets/test_certs/test_ca.pem",
				"api/assets/test_certs/test_wrong_cn_client.pem",
				"api/assets/test_certs/test_client.key",
				logger,
			)
			Expect(err).NotTo(HaveOccurred())

			cmd = newCommandWithConfig(config.Config{
				Address:            listenAddress,
				Port:               listenPort,
				Recursors:          recursorList,
				RecordsFile:        recordsFilePath,
				AddressesFilesGlob: path.Join(addressesDir, "*"),
				AliasFilesGlob:     path.Join(aliasesDir, "*"),
				JobsDir:            jobsDir,
				HandlersFilesGlob:  handlersFilesGlob,
				UpcheckDomains:     []string{"health.check.bosh.", "health.check.ca."},

				API: config.APIConfig{
					Port:            listenAPIPort,
					CAFile:          "api/assets/test_certs/test_ca.pem",
					CertificateFile: "api/assets/test_certs/test_server.pem",
					PrivateKeyFile:  "api/assets/test_certs/test_server.key",
				},

				Health: config.HealthConfig{
					Enabled:         healthEnabled,
					Port:            2345 + ginkgoconfig.GinkgoConfig.ParallelNode,
					CAFile:          "../healthcheck/assets/test_certs/test_ca.pem",
					CertificateFile: "../healthcheck/assets/test_certs/test_client.pem",
					PrivateKeyFile:  "../healthcheck/assets/test_certs/test_client.key",
					CheckInterval:   config.DurationJSON(checkInterval),
				},
			})

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())
			Expect(testhelpers.WaitForListeningTCP(listenAPIPort)).To(Succeed())

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
			Expect(os.RemoveAll(addressesDir)).To(Succeed())

			if httpJSONServer != nil {
				httpJSONServer.Close()
				Expect(os.RemoveAll(handlersDir)).To(Succeed())
			}
		})

		Describe("DNS API", func() {
			itResponds := func(protocol string, addr string) {
				c := &dns.Client{
					Net: protocol,
				}

				m := &dns.Msg{}

				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
				r, _, err := c.Exchange(m, addr)

				Expect(err).NotTo(HaveOccurred())
				Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
			}

			It("responds on the main listen address", func() {
				itResponds("udp", fmt.Sprintf("%s:%d", listenAddress, listenPort))
				itResponds("tcp", fmt.Sprintf("%s:%d", listenAddress, listenPort))
			})

			It("responds to the additional listen addresses", func() {
				itResponds("udp", fmt.Sprintf("%s:%d", listenAddress2, listenPort2))
				itResponds("tcp", fmt.Sprintf("%s:%d", listenAddress2, listenPort2))
			})
		})

		Describe("HTTP API", func() {
			It("returns a 404 from unknown endpoints (well, one unknown endpoint)", func() {
				resp, err := secureGet(apiClient, listenAPIPort, "unknown")
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})

			Describe("/instances", func() {
				var healthServers []*ghttp.Server

				BeforeEach(func() {
					healthServers = []*ghttp.Server{
						newFakeHealthServer("127.0.0.1", "running", nil),
						// sudo ifconfig lo0 alias 127.0.0.2, 3 up # on osx
						newFakeHealthServer("127.0.0.2", "failing", nil),
						newFakeHealthServer("127.0.0.3", "failing", nil),
					}
				})

				AfterEach(func() {
					for _, server := range healthServers {
						server.Close()
					}
				})

				JustBeforeEach(func() {
					c := &dns.Client{Net: "udp"}

					m := &dns.Msg{}
					m.SetQuestion("q-s0.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(2))

					serverRequestLen := func(server *ghttp.Server) func() int {
						return func() int {
							return len(server.ReceivedRequests())
						}
					}
					Eventually(serverRequestLen(healthServers[0]), 5*time.Second).Should(BeNumerically(">", 2))
					Eventually(serverRequestLen(healthServers[1]), 5*time.Second).Should(BeNumerically(">", 2))
				})

				It("returns json records", func() {
					resp, err := secureGet(apiClient, listenAPIPort, "instances")
					Expect(err).NotTo(HaveOccurred())
					defer resp.Body.Close()

					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var parsed []api.InstanceRecord
					decoder := json.NewDecoder(resp.Body)
					var nextRecord api.InstanceRecord
					for decoder.More() {
						err = decoder.Decode(&nextRecord)
						Expect(err).ToNot(HaveOccurred())
						parsed = append(parsed, nextRecord)
					}

					Expect(parsed).To(ConsistOf([]api.InstanceRecord{
						{
							ID:          "my-instance",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment",
							IP:          "127.0.0.1",
							Domain:      "bosh.",
							AZ:          "az1",
							Index:       "",
							HealthState: "running",
						},
						{
							ID:          "my-instance-1",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment",
							IP:          "127.0.0.2",
							Domain:      "bosh.",
							AZ:          "az2",
							Index:       "",
							HealthState: "failing",
						},
						{
							ID:          "my-instance-2",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment-2",
							IP:          "127.0.0.3",
							Domain:      "bosh.",
							AZ:          "az2",
							Index:       "",
							HealthState: "unchecked",
						},
						{
							ID:          "my-instance-3",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment",
							IP:          "127.0.0.2",
							Domain:      "foo.",
							AZ:          "az1",
							Index:       "",
							HealthState: "failing",
						},
						{
							ID:          "my-instance-4",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment-2",
							IP:          "127.0.0.3",
							Domain:      "foo.",
							AZ:          "az2",
							Index:       "",
							HealthState: "unchecked",
						},
						{
							ID:          "primer-instance",
							Group:       "primer-group",
							Network:     "primer-network",
							Deployment:  "primer-deployment",
							IP:          "127.0.0.254",
							Domain:      "primer.",
							AZ:          "az1",
							Index:       "",
							HealthState: "unchecked",
						},
					}))
				})

				It("allows for querying specific addresses without affecting health monitoring", func() {
					Consistently(func() []api.InstanceRecord {
						resp, err := secureGet(apiClient, listenAPIPort, "instances?address=q-s0.my-group.my-network.my-deployment-2.bosh.")
						Expect(err).NotTo(HaveOccurred())
						defer resp.Body.Close()

						Expect(resp.StatusCode).To(Equal(http.StatusOK))

						var parsed []api.InstanceRecord
						decoder := json.NewDecoder(resp.Body)
						var nextRecord api.InstanceRecord
						for decoder.More() {
							err = decoder.Decode(&nextRecord)
							Expect(err).ToNot(HaveOccurred())
							parsed = append(parsed, nextRecord)
						}
						return parsed
					}, 1*time.Second).Should(ConsistOf([]api.InstanceRecord{
						{
							ID:          "my-instance-2",
							Group:       "my-group",
							Network:     "my-network",
							Deployment:  "my-deployment-2",
							IP:          "127.0.0.3",
							Domain:      "bosh.",
							AZ:          "az2",
							Index:       "",
							HealthState: "unchecked",
						},
					}))
				})
			})

			Describe("/local-groups", func() {
				BeforeEach(func() {
					job1Dir := path.Join(jobsDir, "job1", ".bosh")
					err := os.MkdirAll(job1Dir, 0755)
					Expect(err).NotTo(HaveOccurred())

					job2Dir := path.Join(jobsDir, "job2", ".bosh")
					err = os.MkdirAll(job2Dir, 0755)
					Expect(err).NotTo(HaveOccurred())

					ioutil.WriteFile(path.Join(job1Dir, "links.json"), []byte(`[
						{
						  "type": "appetizer",
							"name": "edamame",
							"group": "1"
						},
						{
						  "type": "dessert",
							"name": "yatsuhashi",
							"group": "2"
						}
					]`), 0644)

					ioutil.WriteFile(path.Join(job2Dir, "links.json"), []byte(`[
						{
						  "type": "entree",
							"name": "yakisoba",
							"group": "3"
						}
					]`), 0644)
				})

				Context("with Health enabled", func() {
					var healthServers []*ghttp.Server

					BeforeEach(func() {
						healthServers = []*ghttp.Server{
							newFakeHealthServer("127.0.0.1", "running", map[string]string{
								"1": "running",
								"2": "failing",
								"3": "running",
							}),
						}
					})

					AfterEach(func() {
						for _, server := range healthServers {
							server.Close()
						}
					})

					It("returns group information as JSON", func() {
						resp, err := secureGet(apiClient, listenAPIPort, "local-groups")
						Expect(err).NotTo(HaveOccurred())
						defer resp.Body.Close()

						Expect(resp.StatusCode).To(Equal(http.StatusOK))

						var parsed []api.Group
						decoder := json.NewDecoder(resp.Body)
						var nextGroup api.Group
						for decoder.More() {
							err = decoder.Decode(&nextGroup)
							Expect(err).ToNot(HaveOccurred())
							parsed = append(parsed, nextGroup)
						}
						Expect(parsed).To(ConsistOf([]api.Group{
							{
								JobName:     "",
								LinkType:    "",
								LinkName:    "",
								GroupID:     "",
								HealthState: "running",
							},
							{
								JobName:     "job1",
								LinkType:    "appetizer",
								LinkName:    "edamame",
								GroupID:     "1",
								HealthState: "running",
							},
							{
								JobName:     "job1",
								LinkType:    "dessert",
								LinkName:    "yatsuhashi",
								GroupID:     "2",
								HealthState: "failing",
							},
							{
								JobName:     "job2",
								LinkType:    "entree",
								LinkName:    "yakisoba",
								GroupID:     "3",
								HealthState: "running",
							},
						}))
					})
				})

				Context("with Health disabled", func() {
					BeforeEach(func() {
						healthEnabled = false
					})

					It("returns group information as JSON", func() {
						resp, err := secureGet(apiClient, listenAPIPort, "local-groups")
						Expect(err).NotTo(HaveOccurred())
						defer resp.Body.Close()

						Expect(resp.StatusCode).To(Equal(http.StatusOK))

						var parsed []api.Group
						decoder := json.NewDecoder(resp.Body)
						var nextGroup api.Group
						for decoder.More() {
							err = decoder.Decode(&nextGroup)
							Expect(err).ToNot(HaveOccurred())
							parsed = append(parsed, nextGroup)
						}
						Expect(parsed).To(ConsistOf([]api.Group{
							{
								JobName:     "",
								LinkType:    "",
								LinkName:    "",
								GroupID:     "",
								HealthState: "",
							},
							{
								JobName:     "job1",
								LinkType:    "appetizer",
								LinkName:    "edamame",
								GroupID:     "1",
								HealthState: "",
							},
							{
								JobName:     "job1",
								LinkType:    "dessert",
								LinkName:    "yatsuhashi",
								GroupID:     "2",
								HealthState: "",
							},
							{
								JobName:     "job2",
								LinkType:    "entree",
								LinkName:    "yakisoba",
								GroupID:     "3",
								HealthState: "",
							},
						}))
					})
				})
			})
		})

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

					Context("with glob aliases", func() {
						It("returns all records belonging to the correct group", func() {
							for _, globAlias := range []string{"glob.internal.alias.", "anotherglob.internal.alias.", "yetanotherglob.internal.alias."} {
								m.Question = []dns.Question{{Name: globAlias, Qtype: dns.TypeA}}

								response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
								Expect(err).NotTo(HaveOccurred())

								Expect(response.Answer).To(HaveLen(2))
								Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
								Expect(response.Answer[0].Header().Name).To(Equal(globAlias))
								Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
								Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
								Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))

								Expect(response.Answer[1].Header().Name).To(Equal(globAlias))
								Expect(response.Answer[1].Header().Rrtype).To(Equal(dns.TypeA))
								Expect(response.Answer[1].Header().Class).To(Equal(uint16(dns.ClassINET)))
								Expect(response.Answer[1].Header().Ttl).To(Equal(uint32(0)))

								ips := []string{response.Answer[0].(*dns.A).A.String(), response.Answer[1].(*dns.A).A.String()}
								Expect(ips).To(ConsistOf("127.0.0.1", "127.0.0.2"))

								Eventually(session.Out).Should(
									gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[.*glob\.internal\.alias\.\] 0 \d+ns`),
								)
							}
						})
					})
				})

				Context("with global alias configuration", func() {
					BeforeEach(func() {
						m.SetQuestion("texas.nebraska.", dns.TypeA)
					})

					It("resolves the alias", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal("texas.nebraska."))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[texas\.nebraska\.\] 0 \d+ns`))
					})
				})

				Context("with expanded alias interface", func() {
					BeforeEach(func() {
						m.SetQuestion("my-instance-1.placeholder.alias.", dns.TypeA)
					})

					It("resolves the alias", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal("my-instance-1.placeholder.alias."))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.2"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*INFO \- handlers\.DiscoveryHandler Request \[1\] \[my-instance-1\.placeholder\.alias\.\] 0 \d+ns`))
					})
				})
			})

			Context("upcheck domains", func() {
				BeforeEach(func() {
					m.SetQuestion("health.check.bosh.", dns.TypeA)
				})

				It("responds with a success rcode on the main listen address", func() {
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

				It("responds with a success rcode on the second listen address", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress2, listenPort2))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal("health.check.bosh."))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

					m.SetQuestion("health.check.ca.", dns.TypeA)
					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress2, listenPort2))
					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal("health.check.ca."))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))
				})
			})

			Context("arpa.", func() {
				Context("when arpaing internal ips", func() {
					It("responds with an rcode server failure", func() {
						m.SetQuestion("1.0.0.127.in-addr.arpa.", dns.TypePTR)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(r.Authoritative).To(BeTrue())
						Expect(r.RecursionAvailable).To(BeFalse())
					})
				})

				It("logs handler time", func() {
					m.SetQuestion("1.0.0.127.in-addr.arpa.", dns.TypePTR)
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ArpaHandler Request \[12\] \[1\.0\.0\.127\.in-addr\.arpa\.\] 2 \d+ns`))
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
						m.SetQuestion("my-instance-3.q-g7.foo.", dns.TypeA)

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
					m.SetQuestion("my-instance-3.my-group.my-network.my-deployment.foo.", dns.TypeA)

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
					m.SetQuestion("my-instance-3.my-group.my-network.my-deployment.foo.", dns.TypeA)

					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.DiscoveryHandler Request \[1\] \[my-instance-3\.my-group\.my-network\.my-deployment\.foo\.\] 0 \d+ns`))
				})
			})

			Context("changing records.json", func() {
				JustBeforeEach(func() {
					var err error
					err = ioutil.WriteFile(recordsFilePath, []byte(fmt.Sprint(`{
						"record_keys": ["id", "num_id", "group_ids", "instance_group", "az", "network", "deployment", "ip", "domain"],
						"record_infos": [
							["my-instance", "12", ["19"], "my-group", "az1", "my-network", "my-deployment", "127.0.0.3", "bosh"]
						],
						"aliases": {
							"massachusetts.nebraska": [{
								"group_id": "19",
								"root_domain": "bosh"
							}]
						}
					}`)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("picks up the changes", func() {
					Eventually(func() string {
						c := &dns.Client{}
						m := &dns.Msg{}
						m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(r.Answer).To(HaveLen(1))

						answer := r.Answer[0]
						Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))

						return answer.(*dns.A).A.String()
					}).Should(Equal("127.0.0.3"))

					Eventually(func() []dns.RR {
						c := &dns.Client{}
						m := &dns.Msg{}
						m.SetQuestion("texas.nebraska.", dns.TypeA)
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())
						return response.Answer
					}).Should(HaveLen(0))

					Eventually(func() string {
						c := &dns.Client{}
						m := &dns.Msg{}
						m.SetQuestion("massachusetts.nebraska.", dns.TypeA)
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						if len(response.Answer) == 0 {
							return ""
						}

						return response.Answer[0].(*dns.A).A.String()
					}).Should(Equal("127.0.0.3"))
				})
			})

			Context("http json domains", func() {
				BeforeEach(func() {
					recursorList = []string{"1.1.1.1:1111"}
				})

				Context("when caching is disabled", func() {
					BeforeEach(func() {
						httpJSONServer, handlersDir = setupHttpServer(false, recursorList)
					})

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
				})

				Context("when caching is enabled", func() {
					BeforeEach(func() {
						httpJSONServer, handlersDir = setupHttpServer(true, recursorList)
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

			Context("recursion", func() {
				var (
					server       *dns.Server
					recursorPort int
				)

				BeforeEach(func() {
					var err error
					recursorPort, err = testhelpers.GetFreePort()
					Expect(err).NotTo(HaveOccurred())

					recursorList = []string{
						fmt.Sprintf("127.0.0.1:%d", recursorPort),
					}

					server = &dns.Server{Addr: fmt.Sprintf("0.0.0.0:%d", recursorPort), Net: "tcp", UDPSize: 65535}

					dns.HandleFunc(".", func(resp dns.ResponseWriter, req *dns.Msg) {
						switch name := req.Question[0].Name; name {
						case "test-target.recursor.internal.":
							msg := new(dns.Msg)

							msg.Answer = append(msg.Answer, &dns.A{
								Hdr: dns.RR_Header{
									Name:   req.Question[0].Name,
									Rrtype: dns.TypeA,
									Class:  dns.ClassINET,
									Ttl:    30,
								},
								A: net.ParseIP("192.0.2.100"),
							})

							msg.Authoritative = true
							msg.RecursionAvailable = true

							msg.SetReply(req)
							err := resp.WriteMsg(msg)
							if err != nil {
								Expect(err).NotTo(HaveOccurred())
							}
						case "7.0.0.10.in-addr.arpa.":
							msg := new(dns.Msg)

							msg.Answer = append(msg.Answer, &dns.PTR{
								Hdr: dns.RR_Header{
									Name:   req.Question[0].Name,
									Rrtype: dns.TypePTR,
									Class:  dns.ClassINET,
									Ttl:    0,
								},
								Ptr: "bosh-dns.arpa.com.",
							})

							msg.Authoritative = true
							msg.RecursionAvailable = true

							msg.SetReply(req)

							err := resp.WriteMsg(msg)
							if err != nil {
								Expect(err).NotTo(HaveOccurred())
							}

						case "3.6.8.4.c.e.d.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.7.8.4.1.0.0.2.ip6.arpa.":
							msg := new(dns.Msg)

							msg.Answer = append(msg.Answer, &dns.PTR{
								Hdr: dns.RR_Header{
									Name:   req.Question[0].Name,
									Rrtype: dns.TypePTR,
									Class:  dns.ClassINET,
									Ttl:    0,
								},
								Ptr: "bosh-dns.arpa6.com.",
							})

							msg.Authoritative = true
							msg.RecursionAvailable = true

							msg.SetReply(req)

							err := resp.WriteMsg(msg)
							if err != nil {
								Expect(err).NotTo(HaveOccurred())
							}

						default:
							fmt.Printf("Unexpected request to test recursor :%+v", name)
						}
					})

					go server.ListenAndServe()
					Expect(testhelpers.WaitForListeningTCP(recursorPort)).To(Succeed())
				})

				AfterEach(func() {
					server.Shutdown()
				})

				It("serves local recursor", func() {
					c := &dns.Client{Net: "tcp"}

					m := &dns.Msg{}

					m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))

					answer0 := r.Answer[0].(*dns.A)
					Expect(answer0.A.String()).To(Equal("192.0.2.100"))
				})

				Context("when caching is enabled", func() {
					BeforeEach(func() {
						httpJSONServer, handlersDir = setupHttpServer(true, recursorList)
					})

					It("serves cached responses for local recursor", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						// query the live server
						m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.0.2.100"))

						// make sure the server is really shut down
						server.Shutdown()
						Eventually(func() error {
							m = &dns.Msg{}
							m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
							_, _, err = c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", recursorPort))
							return err
						}, "5s").Should(HaveOccurred())

						// do the same request again and get it from cache
						m = &dns.Msg{}

						m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
						r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 = r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.0.2.100"))
					})
				})

				Context("when caching is disabled", func() {
					It("is unable to lookup recursing answers", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						// query the live server
						m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.0.2.100"))

						// make sure the server is really shut down
						server.Shutdown()
						Eventually(func() error {
							m = &dns.Msg{}
							m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
							_, _, err = c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", recursorPort))
							return err
						}, "5s").Should(HaveOccurred())

						// do the same request again and get it from cache
						m = &dns.Msg{}

						m.SetQuestion("test-target.recursor.internal.", dns.TypeANY)
						r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(r.Answer).To(HaveLen(0))
					})
				})

				Context("arpa.", func() {
					Context("when arpaing external ips", func() {
						It("forwards ipv4 to a recursor", func() {
							c := &dns.Client{Net: "tcp"}

							m := &dns.Msg{}
							reverseAddr, err := dns.ReverseAddr("10.0.0.7")
							Expect(err).NotTo(HaveOccurred())
							m.SetQuestion(reverseAddr, dns.TypePTR)
							r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

							Expect(err).NotTo(HaveOccurred())
							Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(r.Answer).To(HaveLen(1))
							Expect(r.Answer[0].String()).To(MatchRegexp(`\Q7.0.0.10.in-addr.arpa.\E\s+\d+\s+IN\s+PTR\s+\Qbosh-dns.arpa.com.\E`))
						})

						It("forwards ipv6 to a recursor", func() {
							c := &dns.Client{Net: "tcp"}

							m := &dns.Msg{}
							reverseAddr, err := dns.ReverseAddr("2001:4870::1dec:4863")
							Expect(err).NotTo(HaveOccurred())
							m.SetQuestion(reverseAddr, dns.TypePTR)
							r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

							Expect(err).NotTo(HaveOccurred())
							Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(r.Answer).To(HaveLen(1))
							Expect(r.Answer[0].String()).To(MatchRegexp(`\Q3.6.8.4.c.e.d.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.7.8.4.1.0.0.2.ip6.arpa.\E\s+\d+\s+IN\s+PTR\s+\Qbosh-dns.arpa6.com.\E`))
						})
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
						newFakeHealthServer("127.0.0.1", "running", nil),
						// sudo ifconfig lo0 alias 127.0.0.2 up # on osx
						newFakeHealthServer("127.0.0.2", "failing", nil),
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
					checkInterval = time.Minute
					brokenServer = newFakeHealthServer("127.0.0.2", "failing", nil)
					brokenServer.RouteToHandler("GET", "/health", ghttp.RespondWith(http.StatusGatewayTimeout, ``))

					healthServers = []*ghttp.Server{
						newFakeHealthServer("127.0.0.1", "running", nil),
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
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 9000+ginkgoconfig.GinkgoConfig.ParallelNode))
			Expect(err).NotTo(HaveOccurred())
			defer l.Close()

			go func() {
				defer GinkgoRecover()
				_, acceptErr := l.Accept()
				Expect(acceptErr).NotTo(HaveOccurred())
			}()

			cmd = newCommandWithConfig(config.Config{
				Address:         listenAddress,
				Port:            listenPort,
				Recursors:       []string{l.Addr().String()},
				RecursorTimeout: config.DurationJSON(time.Second),
				JobsDir:         jobsDir,

				API: config.APIConfig{
					Port:            listenAPIPort,
					CAFile:          "api/assets/test_certs/test_ca.pem",
					CertificateFile: "api/assets/test_certs/test_server.pem",
					PrivateKeyFile:  "api/assets/test_certs/test_server.key",
				},
			})

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

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

			cmd = newCommandWithConfig(config.Config{
				Address:         listenAddress,
				Port:            listenPort,
				Recursors:       []string{"8.8.8.8"},
				RecursorTimeout: config.DurationJSON(time.Second),
				JobsDir:         jobsDir,

				API: config.APIConfig{
					Port:            listenAPIPort,
					CAFile:          "api/assets/test_certs/test_ca.pem",
					CertificateFile: "api/assets/test_certs/test_server.pem",
					PrivateKeyFile:  "api/assets/test_certs/test_server.key",
				},
			})

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

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
			cmd          *exec.Cmd
			aliasesDir   string
			addressesDir string
		)

		BeforeEach(func() {
			var err error
			aliasesDir, err = ioutil.TempDir("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			addressesDir, err = ioutil.TempDir("", "addresses")
			Expect(err).NotTo(HaveOccurred())

			cmd = newCommandWithConfig(config.Config{
				Address:            listenAddress,
				Port:               listenPort,
				Recursors:          []string{"8.8.8.8"},
				UpcheckDomains:     []string{"upcheck.bosh-dns."},
				AliasFilesGlob:     path.Join(aliasesDir, "*"),
				AddressesFilesGlob: path.Join(addressesDir, "*"),
				JobsDir:            jobsDir,
			})
		})

		AfterEach(func() {
			Expect(os.RemoveAll(aliasesDir)).To(Succeed())
			Expect(os.RemoveAll(addressesDir)).To(Succeed())
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
			cmd = newCommandWithConfig(config.Config{
				Address:        listenAddress,
				Port:           listenPort,
				UpcheckDomains: []string{"upcheck.bosh-dns."},
				Timeout:        config.DurationJSON(-1),
				JobsDir:        jobsDir,

				API: config.APIConfig{
					Port:            listenAPIPort,
					CAFile:          "api/assets/test_certs/test_ca.pem",
					CertificateFile: "api/assets/test_certs/test_server.pem",
					PrivateKeyFile:  "api/assets/test_certs/test_server.key",
				},
			})

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - timed out waiting for server to bind"))
		})

		It("exits 1 and logs a helpful error message when failing to parse jobs", func() {
			cmd = newCommandWithConfig(config.Config{
				Address:        listenAddress,
				Port:           listenPort,
				UpcheckDomains: []string{"upcheck.bosh-dns."},
				Timeout:        config.DurationJSON(-1),
				JobsDir:        "",

				API: config.APIConfig{
					Port:            listenAPIPort,
					CAFile:          "api/assets/test_certs/test_ca.pem",
					CertificateFile: "api/assets/test_certs/test_server.pem",
					PrivateKeyFile:  "api/assets/test_certs/test_server.key",
				},
			})

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - failed to parse jobs directory"))
		})

		Context("mis-configured handlers", func() {
			var handlersDir string
			BeforeEach(func() {
				var err error
				handlersDir, err = ioutil.TempDir("", "handlers")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(handlersDir)).To(Succeed())
			})

			It("exits 1 and logs a helpful error message when the config contains an unknown handler source type", func() {
				writeHandlersConfig(handlersDir, handlersconfig.HandlerConfigs{
					{
						Domain: "internal.domain.",
						Source: handlersconfig.Source{
							Type: "mistyped_dns",
						},
					},
				})

				cmd = newCommandWithConfig(config.Config{
					Address:           listenAddress,
					Port:              listenPort,
					UpcheckDomains:    []string{"upcheck.bosh-dns."},
					HandlersFilesGlob: filepath.Join(handlersDir, "*"),
				})

				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, "5s").Should(gexec.Exit(1))
				Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - Configuring handler for "internal.domain.": Unexpected handler source type: mistyped_dns`))
			})

			It("exits 1 and logs a helpful error message when the dns handler section doesnt contain recursors", func() {
				writeHandlersConfig(handlersDir, handlersconfig.HandlerConfigs{
					{
						Domain: "internal.domain.",
						Source: handlersconfig.Source{
							Type: "dns",
						},
					},
				})

				cmd = newCommandWithConfig(config.Config{
					Address:           listenAddress,
					Port:              listenPort,
					UpcheckDomains:    []string{"upcheck.bosh-dns."},
					HandlersFilesGlob: filepath.Join(handlersDir, "*"),
				})

				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, "5s").Should(gexec.Exit(1))
				Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - Configuring handler for "internal.domain.": No recursors present`))
			})

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

		It("exits 1 and logs a message when the globbed config files contain a broken address config", func() {
			addressesFile1, err := ioutil.TempFile(addressesDir, "addressesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile1.Close()
			_, err = addressesFile1.Write([]byte(`[{"address": "1.2.3.4", "port": 32 }]`))
			Expect(err).NotTo(HaveOccurred())

			addressesFile2, err := ioutil.TempFile(addressesDir, "addressesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile2.Close()
			_, err = addressesFile2.Write([]byte(`{"malformed":"addressesfile"}`))
			Expect(err).NotTo(HaveOccurred())

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - loading addresses configuration:.*addresses config file malformed:`))
			Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf(`addresses config file malformed: %s`, addressesFile2.Name())))
		})
	})
})

func newCommandWithConfig(c config.Config) *exec.Cmd {
	configContents, err := json.Marshal(c)
	Expect(err).NotTo(HaveOccurred())

	configFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(configContents))
	Expect(err).NotTo(HaveOccurred())

	return exec.Command(pathToServer, "--config", configFile.Name())
}

func newFakeHealthServer(ip, state string, groups map[string]string) *ghttp.Server {
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

	port := 2345 + ginkgoconfig.GinkgoConfig.ParallelNode
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

	server.RouteToHandler("GET", "/health", ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
		"state":       state,
		"group_state": groups,
	}))
	server.HTTPTestServer.StartTLS()

	return server
}

func setupHttpServer(handlerCachingEnabled bool, recursorList []string) (*ghttp.Server, string) {
	handlersDir, err := ioutil.TempDir("", "handlers")
	Expect(err).NotTo(HaveOccurred())

	httpJSONServer := ghttp.NewUnstartedServer()
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
	writeHandlersConfig(handlersDir, handlersconfig.HandlerConfigs{
		{
			Domain: "internal-domain.",
			Cache: config.Cache{
				Enabled: handlerCachingEnabled,
			},
			Source: handlersconfig.Source{
				Type: "http",
				URL:  httpJSONServer.URL(),
			},
		}, {
			Domain: "recursor.internal.",
			Cache: config.Cache{
				Enabled: handlerCachingEnabled,
			},
			Source: handlersconfig.Source{
				Type:      "dns",
				Recursors: recursorList,
			},
		},
	})
	return httpJSONServer, handlersDir
}

func writeHandlersConfig(dir string, handlersConfiguration handlersconfig.HandlerConfigs) {
	handlerConfigContents, err := json.Marshal(handlersConfiguration)
	Expect(err).NotTo(HaveOccurred())

	handlersFile, err := ioutil.TempFile(dir, "handlersjson")
	Expect(err).NotTo(HaveOccurred())
	defer handlersFile.Close()

	_, err = handlersFile.Write([]byte(handlerConfigContents))
	Expect(err).NotTo(HaveOccurred())
}

func secureGet(client *httpclient.HTTPClient, port int, path string) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/%s", port, path))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

func localIP() (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:1")
	if err != nil {
		return "", err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return "", err
	}

	defer conn.Close()

	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", err
	}

	return host, nil
}

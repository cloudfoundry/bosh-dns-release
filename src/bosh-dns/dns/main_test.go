package main_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/tlsconfig"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"bosh-dns/dns/api"
	"bosh-dns/dns/config"
	handlersconfig "bosh-dns/dns/config/handlers"
	"bosh-dns/dns/internal/testhelpers"
	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/tlsclient"
)

var _ = Describe("main", func() {
	// up from Ginkgo's Eventually default of 1 Second because of slow test infrastructure
	const expectedTimeout = 5 * time.Second

	var (
		listenAddress  string
		listenPort     int
		listenAddress2 string
		listenPort2    int
		listenAPIPort  int
		recursorList   []string
		jobsDir        string
	)
	var suiteConfig, _ = GinkgoConfiguration()

	BeforeEach(func() {
		listenAddress = "127.0.0.1"
		var err error
		listenPort, err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		listenAPIPort, err = testhelpers.GetFreePort()
		Expect(err).NotTo(HaveOccurred())

		jobsDir, err = os.MkdirTemp("", "jobs")
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

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("--config is a required flag"))
		})

		It("exits 1 if the config file does not exist", func() {
			args := []string{
				"--config",
				"some/fake/path",
			}

			cmd := exec.Command(pathToServer, args...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("Unable to find config file at 'some/fake/path'"))
		})

		It("exits 1 if the config file is busted", func() {
			configFile, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = configFile.Write([]byte("{"))
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(pathToServer, "--config", configFile.Name())
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("unexpected end of JSON input"))
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
			metricsEnabled      bool
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
					["primer-instance", "5", "primer-group", ["9"], "az1", "1", "primer-network", "primer-deployment", "127.0.0.254", "primer"],
					["primer-instance-2", "6", "primer-group", ["10"], "az2", "2", "primer-network", "primer-deployment", "127.0.0.253", "primer"]
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
				},
				"Version": 3,
				"records": [
					["127.0.0.1", "my-instance.my-group.my-network.my-deployment.bosh"],
					["127.0.0.2", "my-instance-1.my-group.my-network.my-deployment.bosh"],
					["127.0.0.3", "my-instance-2.my-group.my-network.my-deployment-2.bosh"],
					["127.0.0.2", "my-instance-3.my-group.my-network.my-deployment.foo"],
					["127.0.0.3", "my-instance-4.my-group.my-network.my-deployment-2.foo"],
					["127.0.0.254", "primer-instance.my-group.my-network.my-deployment.primer"],
					["127.0.0.253", "primer-instance-2.my-group.my-network.my-deployment.primer"]
				]
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
			metricsEnabled = false
			recursorList = []string{}
		})

		JustBeforeEach(func() {
			var err error

			recordsFile, err := os.CreateTemp("", "recordsjson")
			Expect(err).NotTo(HaveOccurred())

			_, err = recordsFile.Write([]byte(recordsJSONContent))
			Expect(err).NotTo(HaveOccurred())

			recordsFilePath = recordsFile.Name()

			addressesDir, err = os.MkdirTemp("", "addresses")
			Expect(err).NotTo(HaveOccurred())

			listenAddress2, err = localIP()
			Expect(err).NotTo(HaveOccurred())

			listenPort2, err = testhelpers.GetFreePort()
			Expect(err).NotTo(HaveOccurred())

			addressesFile, err := os.CreateTemp(addressesDir, "addresses")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile.Close()                                                                                        //nolint:errcheck
			_, err = addressesFile.Write([]byte(fmt.Sprintf(`[{"address": "%s", "port": %d }]`, listenAddress2, listenPort2))) //nolint:staticcheck
			Expect(err).NotTo(HaveOccurred())

			aliasesDir, err = os.MkdirTemp("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			aliasesFile1, err := os.CreateTemp(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close() //nolint:errcheck
			_, err = aliasesFile1.Write([]byte(aliases1JSONContent))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := os.CreateTemp(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close() //nolint:errcheck
			_, err = aliasesFile2.Write([]byte(aliases2JSONContent))
			Expect(err).NotTo(HaveOccurred())

			var handlersFilesGlob string
			if handlersDir != "" {
				handlersFilesGlob = path.Join(handlersDir, "*")
			}

			logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, io.Discard)
			apiClient, err = tlsclient.NewFromFiles(
				"api.bosh-dns",
				"api/assets/test_certs/test_ca.pem",
				"api/assets/test_certs/test_wrong_cn_client.pem",
				"api/assets/test_certs/test_client.key",
				1*time.Second,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())

			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.Recursors = recursorList
			cfg.RecordsFile = recordsFilePath
			cfg.AddressesFilesGlob = path.Join(addressesDir, "*")
			cfg.AliasFilesGlob = path.Join(aliasesDir, "*")
			cfg.JobsDir = jobsDir
			cfg.HandlersFilesGlob = handlersFilesGlob
			cfg.UpcheckDomains = []string{"health.check.bosh.", "health.check.ca."}

			cfg.API = config.APIConfig{
				Port:            listenAPIPort,
				CAFile:          "api/assets/test_certs/test_ca.pem",
				CertificateFile: "api/assets/test_certs/test_server.pem",
				PrivateKeyFile:  "api/assets/test_certs/test_server.key",
			}

			cfg.Health = config.HealthConfig{
				Enabled:         healthEnabled,
				Port:            2345 + suiteConfig.ParallelProcess,
				CAFile:          "../healthcheck/assets/test_certs/test_ca.pem",
				CertificateFile: "../healthcheck/assets/test_certs/test_client.pem",
				PrivateKeyFile:  "../healthcheck/assets/test_certs/test_client.key",
				CheckInterval:   config.DurationJSON(checkInterval),
			}

			cfg.Metrics = config.MetricsConfig{
				Enabled: metricsEnabled,
				Address: "127.0.0.1",
				Port:    53088 + suiteConfig.ParallelProcess,
			}

			cmd = newCommandWithConfig(cfg)

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())
			Expect(testhelpers.WaitForListeningTCP(listenAPIPort)).To(Succeed())

			Eventually(func() int {
				c := &dns.Client{}
				m := &dns.Msg{}
				SetQuestion(m, nil, "q-s0.primer-group.primer-network.primer-deployment.primer.", dns.TypeANY)
				r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
				if err != nil {
					return -1
				}

				return r.Rcode
			}, 5*time.Second).Should(Equal(dns.RcodeSuccess))
		})

		AfterEach(func() {
			if session != nil {
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

				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
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
				defer resp.Body.Close() //nolint:errcheck

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
					SetQuestion(m, nil, "q-s0.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(2))

					serverRequestLen := func(server *ghttp.Server) func() int {
						return func() int {
							return len(server.ReceivedRequests())
						}
					}
					Eventually(serverRequestLen(healthServers[0]), 3*time.Second).Should(BeNumerically(">", 2))
					Eventually(serverRequestLen(healthServers[1]), 3*time.Second).Should(BeNumerically(">", 2))
				})

				It("returns json records", func() {
					status := func() []api.InstanceRecord {
						resp, err := secureGet(apiClient, listenAPIPort, "instances")
						Expect(err).NotTo(HaveOccurred())
						defer resp.Body.Close() //nolint:errcheck

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
					}

					Eventually(status, 10*time.Second).Should(ConsistOf([]api.InstanceRecord{
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
							HealthState: "unknown",
						},
						{
							ID:          "primer-instance-2",
							Group:       "primer-group",
							Network:     "primer-network",
							Deployment:  "primer-deployment",
							IP:          "127.0.0.253",
							Domain:      "primer.",
							AZ:          "az2",
							Index:       "",
							HealthState: "unknown",
						},
					}))
				})

				It("allows for querying specific addresses without affecting health monitoring", func() {
					Consistently(func() []api.InstanceRecord {
						resp, err := secureGet(apiClient, listenAPIPort, "instances?address=q-s0.my-group.my-network.my-deployment-2.bosh.")
						Expect(err).NotTo(HaveOccurred())
						defer resp.Body.Close() //nolint:errcheck

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
					}, 1*time.Second, 500*time.Millisecond).Should(ConsistOf([]api.InstanceRecord{
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

					Expect(os.WriteFile(path.Join(job1Dir, "links.json"),
						[]byte(`[
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
					]`), 0644)).To(Succeed())

					Expect(os.WriteFile(path.Join(job2Dir, "links.json"),
						[]byte(`[
						{
						  "type": "entree",
							"name": "yakisoba",
							"group": "3"
						}
					]`), 0644)).To(Succeed())
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
						defer resp.Body.Close() //nolint:errcheck

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
						defer resp.Body.Close() //nolint:errcheck

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

				Context("with Metrics enabled", func() {
					BeforeEach(func() {
						metricsEnabled = true
					})

					It("starts the metrics server and collects metrics for internal resolutions", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						SetQuestion(m, nil, "group.internal.alias.", dns.TypeANY)
						_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						resp, err := http.Get(fmt.Sprintf("http://%s:%d/metrics", listenAddress, 53088+suiteConfig.ParallelProcess))
						Expect(err).NotTo(HaveOccurred())

						metrics, err := io.ReadAll(resp.Body)
						defer resp.Body.Close() //nolint:errcheck
						Expect(err).NotTo(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusOK))
						Expect(string(metrics)).To(MatchRegexp("coredns_dns_requests_total{family=\"1\",proto=\"tcp\",server=\"\",type=\"ANY\",view=\"\",zone=\".\"} [1-9][0-9]*"))
					})
				})

				Context("with Metrics disabled", func() {

					It("does not start the metrics server", func() {
						_, err := http.Get(fmt.Sprintf("http://%s:%d/metrics", listenAddress, 53088+suiteConfig.ParallelProcess))

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(MatchRegexp("connection.*refused"))
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
					var casedQname string
					BeforeEach(func() {
						SetQuestion(m, &casedQname, "one.alias.", dns.TypeA)
					})

					It("resolves to the appropriate domain before deferring to mux", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal(casedQname))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
					})
				})

				Context("with an address resolving to an IP", func() {
					var casedQname string
					BeforeEach(func() {
						SetQuestion(m, &casedQname, "ip.alias.", dns.TypeA)
					})

					It("resolves to the appropriate domain before deferring to mux", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal(casedQname))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("10.11.12.13"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
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

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[internal\.alias\.\] rcode=NOERROR ancount=2 time=\d+ns`))
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

							Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[group\.internal\.alias\.\] rcode=NOERROR ancount=2 time=\d+ns`))
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
									gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[.*glob\.internal\.alias\.\] rcode=NOERROR ancount=2 time=\d+ns`),
								)
							}
						})
					})
				})

				Context("with global alias configuration", func() {
					var casedQname string
					BeforeEach(func() {
						SetQuestion(m, &casedQname, "texas.nebraska.", dns.TypeA)
					})

					It("resolves the alias", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal(casedQname))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
					})
				})

				Context("with expanded alias interface", func() {
					var casedQname string
					BeforeEach(func() {
						SetQuestion(m, &casedQname, "my-instance-1.placeholder.alias.", dns.TypeA)
					})

					It("resolves the alias", func() {
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Answer).To(HaveLen(1))
						Expect(response.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(response.Answer[0].Header().Name).To(Equal(casedQname))
						Expect(response.Answer[0].Header().Rrtype).To(Equal(dns.TypeA))
						Expect(response.Answer[0].Header().Class).To(Equal(uint16(dns.ClassINET)))
						Expect(response.Answer[0].Header().Ttl).To(Equal(uint32(0)))
						Expect(response.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.2"))

						Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*DEBUG \- handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
					})
				})
			})

			Context("upcheck domains", func() {
				var casedQname string
				BeforeEach(func() {
					SetQuestion(m, &casedQname, "health.check.bosh.", dns.TypeA)
				})

				It("responds with a success rcode on the main listen address", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal(casedQname))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

					SetQuestion(m, &casedQname, "health.check.ca.", dns.TypeA)
					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal(casedQname))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))
				})

				It("responds with a success rcode on the second listen address", func() {
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress2, listenPort2))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal(casedQname))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))

					SetQuestion(m, &casedQname, "health.check.ca.", dns.TypeA)
					r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress2, listenPort2))
					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))
					Expect(r.Answer[0].(*dns.A).Header().Name).To(Equal(casedQname))
					Expect(r.Answer[0].(*dns.A).A.String()).To(Equal("127.0.0.1"))
				})
			})

			Context("arpa.", func() {
				Context("when arpaing internal ips", func() {
					It("responds with an rcode success", func() {
						SetQuestion(m, nil, "254.0.0.127.in-addr.arpa.", dns.TypePTR)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Authoritative).To(BeTrue())
						Expect(r.RecursionAvailable).To(BeFalse())
						Expect(len(r.Answer)).To(Equal(1))
						Expect(r.Answer[0].(*dns.PTR).Ptr).To(Equal("primer-instance.my-group.my-network.my-deployment.primer."))
					})

					It("responds with alias records", func() {
						SetQuestion(m, nil, "1.0.0.127.in-addr.arpa.", dns.TypePTR)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(len(r.Answer)).To(Equal(3))
						for _, answer := range r.Answer {
							Expect(answer.(*dns.PTR).Ptr).Should(BeElementOf([]string{
								"my-instance.my-group.my-network.my-deployment.bosh.",
								"one.alias.",
								"internal.alias."}))
						}
					})
				})

				It("logs handler time", func() {
					var casedQname string
					SetQuestion(m, &casedQname, "254.0.0.127.in-addr.arpa.", dns.TypePTR)
					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ArpaHandler Request id=\d+ qtype=\[PTR\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
				})
			})

			Context("domains from records.json", func() {
				It("can interpret AZ-specific queries", func() {
					SetQuestion(m, nil, "q-a1s0.my-group.my-network.my-deployment.bosh.", dns.TypeA)

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

					SetQuestion(m, nil, "q-a2s0.my-group.my-network.my-deployment.bosh.", dns.TypeA)

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
						SetQuestion(m, nil, "q-a1s0.q-g7.foo.", dns.TypeA)

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
						SetQuestion(m, nil, "my-instance-3.q-g7.foo.", dns.TypeA)

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
					SetQuestion(m, nil, "my-instance-3.my-group.my-network.my-deployment.foo.", dns.TypeA)

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
					var casedQname string
					SetQuestion(m, &casedQname, "my-instance-3.my-group.my-network.my-deployment.foo.", dns.TypeA)

					_, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
					Expect(err).NotTo(HaveOccurred())

					Eventually(session.Out).Should(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.DiscoveryHandler Request id=\d+ qtype=\[A\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=1 time=\d+ns`))
				})
			})

			Context("changing records.json", func() {
				JustBeforeEach(func() {
					err := os.WriteFile(recordsFilePath, []byte(`{
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
					}`), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("picks up the changes", func() {
					Eventually(func() string {
						c := &dns.Client{}
						m := &dns.Msg{}
						SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())

						Expect(r.Answer).To(HaveLen(1))

						answer := r.Answer[0]
						Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))

						return answer.(*dns.A).A.String()
					}, 5*time.Second).Should(Equal("127.0.0.3"))

					Eventually(func() []dns.RR {
						c := &dns.Client{}
						m := &dns.Msg{}
						SetQuestion(m, nil, "texas.nebraska.", dns.TypeA)
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						Expect(err).NotTo(HaveOccurred())
						return response.Answer
					}).Should(HaveLen(0))

					Eventually(func() string {
						c := &dns.Client{}
						m := &dns.Msg{}
						SetQuestion(m, nil, "massachusetts.nebraska.", dns.TypeA)
						response, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
						if err != nil {
							return ""
						}

						if len(response.Answer) == 0 {
							return ""
						}

						return response.Answer[0].(*dns.A).A.String()
					}, 5*time.Second).Should(Equal("127.0.0.3"))
				})
			})

			Context("http json domains", func() {
				BeforeEach(func() {
					recursorList = []string{"1.1.1.1:1111"}
				})

				Context("when caching is disabled", func() {
					BeforeEach(func() {
						httpJSONServer, handlersDir = setupHttpServer(MixCase("app-id.internal-domain."), false, recursorList)
					})

					It("serves the addresses from the http server", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						SetQuestion(m, nil, "app-id.internal-domain.", dns.TypeANY)
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
						httpJSONServer, handlersDir = setupHttpServer(MixCase("app-id.internal-domain."), true, recursorList)
					})

					It("should return cached answers", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						SetQuestion(m, nil, "app-id.internal-domain.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.168.0.1"))

						m = &dns.Msg{}

						SetQuestion(m, nil, "app-id.internal-domain.", dns.TypeANY)
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
						lowercaseName := strings.ToLower(req.Question[0].Name)
						switch name := lowercaseName; name {
						case "test-target.recursor.internal.":
							msg := new(dns.Msg)

							msg.Answer = append(msg.Answer, &dns.A{
								Hdr: dns.RR_Header{
									Name:   lowercaseName,
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
									Name:   lowercaseName,
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
									Name:   lowercaseName,
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

					go func() {
						defer GinkgoRecover()
						err := server.ListenAndServe()
						Expect(err).NotTo(HaveOccurred())
					}()
					Expect(testhelpers.WaitForListeningTCP(recursorPort)).To(Succeed())
				})

				AfterEach(func() {
					// ignore error, server may have already been shutdown
					server.Shutdown() //nolint:errcheck
				})

				It("serves local recursor", func() {
					c := &dns.Client{Net: "tcp"}

					m := &dns.Msg{}

					SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
					r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

					Expect(err).NotTo(HaveOccurred())
					Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(r.Answer).To(HaveLen(1))

					answer0 := r.Answer[0].(*dns.A)
					Expect(answer0.A.String()).To(Equal("192.0.2.100"))
				})

				Context("when caching is enabled", func() {
					BeforeEach(func() {
						httpJSONServer, handlersDir = setupHttpServer(MixCase("test-target.recursor.internal."), true, recursorList)
					})

					It("serves cached responses for local recursor", func() {
						c := &dns.Client{Net: "tcp"}

						m := &dns.Msg{}

						// query the live server
						SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.0.2.100"))

						// make sure the server is really shut down
						Expect(server.Shutdown()).To(Succeed())
						Eventually(func() error {
							m = &dns.Msg{}
							SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
							_, _, err = c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", recursorPort))
							return err
						}, "5s").Should(HaveOccurred())

						// do the same request again and get it from cache
						m = &dns.Msg{}

						SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
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
						SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
						r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(r.Answer).To(HaveLen(1))

						answer0 := r.Answer[0].(*dns.A)
						Expect(answer0.A.String()).To(Equal("192.0.2.100"))

						// make sure the server is really shut down
						Expect(server.Shutdown()).To(Succeed())
						Eventually(func() error {
							m = &dns.Msg{}
							SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
							_, _, err = c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", recursorPort))
							return err
						}, "5s").Should(HaveOccurred())

						// do the same request again and get it from cache
						m = &dns.Msg{}

						SetQuestion(m, nil, "test-target.recursor.internal.", dns.TypeANY)
						r, _, err = c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

						Expect(err).NotTo(HaveOccurred())
						Expect(r.Rcode).To(Equal(dns.RcodeNameError))
						Expect(r.Answer).To(HaveLen(0))
					})
				})

				Context("arpa.", func() {
					Context("when arpaing external ips", func() {
						It("forwards ipv4 to a recursor", func() {
							var casedQname string
							c := &dns.Client{Net: "tcp"}

							m := &dns.Msg{}
							reverseAddr, err := dns.ReverseAddr("10.0.0.7")
							Expect(err).NotTo(HaveOccurred())
							SetQuestion(m, &casedQname, reverseAddr, dns.TypePTR)
							r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))

							Expect(err).NotTo(HaveOccurred())
							Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(r.Answer).To(HaveLen(1))
							Expect(r.Answer[0].String()).To(MatchRegexp(`\Q7.0.0.10.in-addr.arpa.\E\s+\d+\s+IN\s+PTR\s+\Qbosh-dns.arpa.com.\E`))
						})

						It("forwards ipv6 to a recursor", func() {
							var casedQname string
							c := &dns.Client{Net: "tcp"}

							m := &dns.Msg{}
							reverseAddr, err := dns.ReverseAddr("2001:4870::1dec:4863")
							Expect(err).NotTo(HaveOccurred())
							SetQuestion(m, &casedQname, reverseAddr, dns.TypePTR)
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

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(0))
		})

		DescribeTable("local horizontal scaling (network buffers)",
			func(protocol string) {
				wg := &sync.WaitGroup{}
				concurrentRequests := 200
				attempts := 5
				c := &dns.Client{Net: protocol}
				m := &dns.Msg{}
				SetQuestion(m, nil, "primer-instance.primer-group.primer-network.primer-deployment.primer.", dns.TypeANY)

				for j := 0; j < attempts; j++ {
					wg.Add(concurrentRequests)
					for i := 0; i < concurrentRequests; i++ {
						go func(wg *sync.WaitGroup, m dns.Msg) {
							defer GinkgoRecover()
							defer wg.Done()
							r, _, err := c.Exchange(&m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
							Expect(err).ToNot(HaveOccurred(),
								fmt.Sprintln(`
===========================================
 ATTENTION: on macOS you may need to run
    sudo sysctl -w kern.ipc.somaxconn=1024 #default is 128
===========================================`))
							Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
						}(wg, *m)
					}
					wg.Wait()
				}

			},
			Entry("over udp", "udp"),
			Entry("over tcp", "tcp"),
		)

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
					SetQuestion(m, nil, "q-s0.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

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

					err = os.WriteFile(recordsFilePath, []byte(`{
						"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
						"record_infos": [
							["my-instance", "my-group", "az1", "my-network", "my-deployment", "127.0.0.1", "bosh"]
						]
					}`), 0644)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() bool {
						startLength := len(healthServers[1].ReceivedRequests())
						time.Sleep(200 * time.Millisecond)
						finalLength := len(healthServers[1].ReceivedRequests())
						return startLength == finalLength
					}, 5*time.Second).Should(BeTrue())
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
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 9000+suiteConfig.ParallelProcess))
			Expect(err).NotTo(HaveOccurred())
			defer l.Close() //nolint:errcheck

			go func() {
				defer GinkgoRecover()
				_, acceptErr := l.Accept()
				Expect(acceptErr).NotTo(HaveOccurred())
			}()

			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.Recursors = []string{l.Addr().String()}
			cfg.RecursorTimeout = config.DurationJSON(time.Second)
			cfg.JobsDir = jobsDir

			cfg.API = config.APIConfig{
				Port:            listenAPIPort,
				CAFile:          "api/assets/test_certs/test_ca.pem",
				CertificateFile: "api/assets/test_certs/test_server.pem",
				PrivateKeyFile:  "api/assets/test_certs/test_server.key",
			}
			cmd = newCommandWithConfig(cfg)

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

			timeoutNeverToBeReached := 10 * time.Second
			c := &dns.Client{
				Net:     "tcp",
				Timeout: timeoutNeverToBeReached,
			}

			m := &dns.Msg{}

			var casedQname string
			SetQuestion(m, &casedQname, "bosh.io.", dns.TypeANY)

			startTime := time.Now()
			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
			Expect(time.Since(startTime)).Should(BeNumerically(">", 999*time.Millisecond))
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeNameError))

			Eventually(session.Out).Should(gbytes.Say(`\[ForwardHandler\].*handlers\.ForwardHandler Request id=\d+ qtype=\[ANY\] qname=\[` + casedQname + `\] rcode=NXDOMAIN ancount=0 error=\[no response from recursors\] time=\d+ns`))
		})

		It("logs the recursor used to resolve", func() {
			var err error

			recursorAddress := os.Getenv("BOSH_DNS_RECURSOR_ADDRESS")
			if recursorAddress == "" {
				recursorAddress = "169.254.169.254"
			}

			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.Recursors = []string{recursorAddress}
			cfg.RecursorTimeout = config.DurationJSON(time.Second)
			cfg.JobsDir = jobsDir
			cfg.API = config.APIConfig{
				Port:            listenAPIPort,
				CAFile:          "api/assets/test_certs/test_ca.pem",
				CertificateFile: "api/assets/test_certs/test_server.pem",
				PrivateKeyFile:  "api/assets/test_certs/test_server.key",
			}
			cmd = newCommandWithConfig(cfg)

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Expect(testhelpers.WaitForListeningTCP(listenPort)).To(Succeed())

			c := &dns.Client{}
			m := &dns.Msg{}
			var casedQname string
			SetQuestion(m, &casedQname, "bosh.io.", dns.TypeANY)

			r, _, err := c.Exchange(m, fmt.Sprintf("%s:%d", listenAddress, listenPort))
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Rcode).To(Equal(dns.RcodeSuccess))

			Eventually(session.Out).Should(gbytes.Say(`\[ForwardHandler\].*handlers\.ForwardHandler Request id=\d+ qtype=\[ANY\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=\d+ recursor=` + recursorAddress + `:53\ time=\d+ns`))
			Consistently(session.Out).ShouldNot(gbytes.Say(`\[RequestLoggerHandler\].*handlers\.ForwardHandler Request id=\d+ qtype=\[ANY\] qname=\[` + casedQname + `\] rcode=NOERROR ancount=\d+ time=\d+ns`))
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
			aliasesDir, err = os.MkdirTemp("", "aliases")
			Expect(err).NotTo(HaveOccurred())

			addressesDir, err = os.MkdirTemp("", "addresses")
			Expect(err).NotTo(HaveOccurred())

			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.Recursors = []string{"169.254.169.254"}
			cfg.UpcheckDomains = []string{"upcheck.bosh-dns."}
			cfg.AliasFilesGlob = path.Join(aliasesDir, "*")
			cfg.AddressesFilesGlob = path.Join(addressesDir, "*")
			cfg.JobsDir = jobsDir
			cmd = newCommandWithConfig(cfg)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(aliasesDir)).To(Succeed())
			Expect(os.RemoveAll(addressesDir)).To(Succeed())
		})

		It("exits 1 when fails to bind to the tcp port", func() {
			listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
			Expect(err).NotTo(HaveOccurred())
			defer listener.Close() //nolint:errcheck

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
		})

		It("exits 1 when fails to bind to the udp port", func() {
			listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(listenAddress), Port: listenPort})
			Expect(err).NotTo(HaveOccurred())
			defer listener.Close() //nolint:errcheck

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
		})

		It("exits 1 and logs a helpful error message when the server times out binding to ports", func() {
			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.UpcheckDomains = []string{"upcheck.bosh-dns."}
			cfg.BindTimeout = config.DurationJSON(-1)
			cfg.JobsDir = jobsDir

			cfg.API = config.APIConfig{
				Port:            listenAPIPort,
				CAFile:          "api/assets/test_certs/test_ca.pem",
				CertificateFile: "api/assets/test_certs/test_server.pem",
				PrivateKeyFile:  "api/assets/test_certs/test_server.key",
			}
			cmd = newCommandWithConfig(cfg)

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - bosh-dns failed: timed out waiting for server to bind"))
		})

		It("exits 1 and logs a helpful error message when failing to parse jobs", func() {
			cfg := config.NewDefaultConfig()
			cfg.Address = listenAddress
			cfg.Port = listenPort
			cfg.UpcheckDomains = []string{"upcheck.bosh-dns."}
			cfg.BindTimeout = config.DurationJSON(-1)
			cfg.JobsDir = ""

			cfg.API = config.APIConfig{
				Port:            listenAPIPort,
				CAFile:          "api/assets/test_certs/test_ca.pem",
				CertificateFile: "api/assets/test_certs/test_server.pem",
				PrivateKeyFile:  "api/assets/test_certs/test_server.key",
			}
			cmd = newCommandWithConfig(cfg)

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "5s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say("[main].*ERROR - failed to parse jobs directory"))
		})

		Context("mis-configured handlers", func() {
			var handlersDir string
			BeforeEach(func() {
				var err error
				handlersDir, err = os.MkdirTemp("", "handlers")
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

				cfg := config.NewDefaultConfig()
				cfg.Address = listenAddress
				cfg.Port = listenPort
				cfg.UpcheckDomains = []string{"upcheck.bosh-dns."}
				cfg.HandlersFilesGlob = filepath.Join(handlersDir, "*")
				cmd = newCommandWithConfig(cfg)

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

				cfg := config.NewDefaultConfig()
				cfg.Address = listenAddress
				cfg.Port = listenPort
				cfg.UpcheckDomains = []string{"upcheck.bosh-dns."}
				cfg.HandlersFilesGlob = filepath.Join(handlersDir, "*")
				cmd = newCommandWithConfig(cfg)

				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, "5s").Should(gexec.Exit(1))
				Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - Configuring handler for "internal.domain.": No recursors present`))
			})

		})

		It("exits 1 and logs a message when the globbed config files contain a broken alias config", func() {
			aliasesFile1, err := os.CreateTemp(aliasesDir, "aliasesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile1.Close() //nolint:errcheck
			_, err = aliasesFile1.Write([]byte(`{
				"uc.alias.": ["upcheck.bosh-dns."]
			}`))
			Expect(err).NotTo(HaveOccurred())

			aliasesFile2, err := os.CreateTemp(aliasesDir, "aliasesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer aliasesFile2.Close() //nolint:errcheck
			_, err = aliasesFile2.Write([]byte(`{"malformed":"aliasfile"}`))
			Expect(err).NotTo(HaveOccurred())

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - loading alias configuration:.*alias config file malformed:`))
			Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf(`alias config file malformed: %s`, aliasesFile2.Name())))
		})

		It("exits 1 and logs a message when the globbed config files contain a broken address config", func() {
			addressesFile1, err := os.CreateTemp(addressesDir, "addressesjson1")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile1.Close() //nolint:errcheck
			_, err = addressesFile1.Write([]byte(`[{"address": "1.2.3.4", "port": 32 }]`))
			Expect(err).NotTo(HaveOccurred())

			addressesFile2, err := os.CreateTemp(addressesDir, "addressesjson2")
			Expect(err).NotTo(HaveOccurred())
			defer addressesFile2.Close() //nolint:errcheck
			_, err = addressesFile2.Write([]byte(`{"malformed":"addressesfile"}`))
			Expect(err).NotTo(HaveOccurred())

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).WithTimeout(expectedTimeout).Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say(`[main].*ERROR - loading addresses configuration:.*addresses config file malformed:`))
			Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf(`addresses config file malformed: %s`, addressesFile2.Name())))
		})
	})
})

func newCommandWithConfig(c config.Config) *exec.Cmd {
	configContents, err := json.Marshal(c)
	Expect(err).NotTo(HaveOccurred())

	configFile, err := os.CreateTemp("", "")
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write([]byte(configContents))
	Expect(err).NotTo(HaveOccurred())

	return exec.Command(pathToServer, "--config", configFile.Name())
}

func newFakeHealthServer(ip, state string, groups map[string]string) *ghttp.Server {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithIdentityFromFile("../healthcheck/assets/test_certs/test_server.pem", "../healthcheck/assets/test_certs/test_server.key"),
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile("../healthcheck/assets/test_certs/test_ca.pem"),
	)
	Expect(err).ToNot(HaveOccurred())

	server := ghttp.NewUnstartedServer()
	err = server.HTTPTestServer.Listener.Close()
	Expect(err).NotTo(HaveOccurred())

	suiteConfig, _ := GinkgoConfiguration()
	port := 2345 + suiteConfig.ParallelProcess
	server.HTTPTestServer.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	Expect(err).ToNot(HaveOccurred(),
		fmt.Sprintf(`
===========================================
 ATTENTION: on macOS you may need to run
    sudo ifconfig lo0 alias %s up
===========================================
`, ip),
	)

	server.HTTPTestServer.TLS = tlsConfig

	server.RouteToHandler("GET", "/health", ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
		"state":       state,
		"group_state": groups,
	}))
	server.HTTPTestServer.StartTLS()

	return server
}

func setupHttpServer(qname string, handlerCachingEnabled bool, recursorList []string) (*ghttp.Server, string) {
	handlersDir, err := os.MkdirTemp("", "handlers")
	Expect(err).NotTo(HaveOccurred())

	httpJSONServer := ghttp.NewUnstartedServer()
	httpJSONServer.AppendHandlers(ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/", "name="+qname+"&type=255"),
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
      "name": "`+qname+`",
      "type": 28
    }
  ],
  "Answer":
  [
    {
      "name": "`+qname+`",
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

	handlersFile, err := os.CreateTemp(dir, "handlersjson")
	Expect(err).NotTo(HaveOccurred())
	defer handlersFile.Close() //nolint:errcheck

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

	defer conn.Close() //nolint:errcheck

	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", err
	}

	return host, nil
}

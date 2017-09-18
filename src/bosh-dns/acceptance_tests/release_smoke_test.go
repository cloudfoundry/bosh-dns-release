package acceptance_test

import (
	"bosh-dns/healthcheck/healthclient"
	"crypto/tls"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"encoding/json"
	"io/ioutil"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"net/http"
	"path/filepath"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

var _ = Describe("Integration", func() {
	var firstInstance instanceInfo

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDnsRelease()
			firstInstance = allDeployedInstances[0]
		})

		It("returns records for bosh instances", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A %s.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.InstanceID, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Eventually(session.Out).Should(gbytes.Say(
				"%s\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s",
				firstInstance.InstanceID,
				firstInstance.IP))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns records for bosh instances found with query for all records", func() {
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns records for bosh instances found with query for index", func() {
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-i%s.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.Index, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("q-i%s\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", firstInstance.Index, firstInstance.IP))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("finds and resolves aliases specified in other jobs on the same instance", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A A internal.alias. @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("should resolve specified upcheck", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A A upcheck.bosh-dns. @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})

	Context("Instance health", func() {
		BeforeEach(func() {
			ensureHealthEndpointDeployed()
			firstInstance = allDeployedInstances[0]
		})

		It("returns a healthy response when the instance is running", func() {
			client := setupSecureGet()

			Eventually(func() map[string]string {
				respData, err := secureGetRespBody(client, firstInstance.IP, 2345)
				Expect(err).ToNot(HaveOccurred())

				var respJson map[string]string
				err = json.Unmarshal(respData, &respJson)
				Expect(err).ToNot(HaveOccurred())
				return respJson
			}, 31*time.Second).Should(Equal(map[string]string{
				"state": "running",
			}))
		})

		It("stops returning IP addresses of instances that go unhealthy", func() {
			var output string
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))

			output = string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))

			stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "-n", "-d", boshDeployment,
				"stop", fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))

			defer func() {
				stdOut, stdErr, exitStatus, err = cmdRunner.RunCommand(boshBinaryPath, "-n", "-d", boshDeployment,
					"start", fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
			}()

			Eventually(func() string {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))

				output = string(session.Out.Contents())

				return output
			}, 60*time.Second, 1*time.Second).Should(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			// ^ timeout = agent heartbeat updates health.json every 20s + dns checks healthiness every 20s + a buffer interval

			Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", firstInstance.IP))
			Expect(output).ToNot(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", allDeployedInstances[1].IP))
		})

		Context("when a job defines a healthy executable", func() {
			var (
				osSuffix string
			)
			BeforeEach(func() {
				osSuffix = ""
				if testTargetOS == "windows" {
					osSuffix = "-windows"
				}
				ensureHealthEndpointDeployed("-o", "../test_yml_assets/enable-healthy-executable-job"+osSuffix+".yml")
				firstInstance = allDeployedInstances[0]
			})

			It("changes the health endpoint return value based on how the executable exits", func() {
				client := setupSecureGet()

				Eventually(func() map[string]string {
					respData, err := secureGetRespBody(client, firstInstance.IP, 2345)
					Expect(err).ToNot(HaveOccurred())

					var respJson map[string]string
					err = json.Unmarshal(respData, &respJson)
					Expect(err).ToNot(HaveOccurred())
					return respJson
				}, 31*time.Second).Should(Equal(map[string]string{
					"state": "running",
				}))

				runErrand("make-health-executable-job-unhealthy" + osSuffix)

				Eventually(func() map[string]string {
					respData, err := secureGetRespBody(client, firstInstance.IP, 2345)
					Expect(err).ToNot(HaveOccurred())

					var respJson map[string]string
					err = json.Unmarshal(respData, &respJson)
					Expect(err).ToNot(HaveOccurred())
					return respJson
				}, 31*time.Second).Should(Equal(map[string]string{
					"state": "job-health-executable-fail",
				}))

				runErrand("make-health-executable-job-healthy" + osSuffix)

				Eventually(func() map[string]string {
					respData, err := secureGetRespBody(client, firstInstance.IP, 2345)
					Expect(err).ToNot(HaveOccurred())

					var respJson map[string]string
					err = json.Unmarshal(respData, &respJson)
					Expect(err).ToNot(HaveOccurred())
					return respJson
				}, 31*time.Second).Should(Equal(map[string]string{
					"state": "running",
				}))
			})
		})
	})
})

func runErrand(errandName string) {
	session, err := gexec.Start(exec.Command(
		boshBinaryPath, "-n",
		"-d", boshDeployment,
		"run-errand", errandName,
	), GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session, time.Minute).Should(gexec.Exit(0))
}

func ensureHealthEndpointDeployed(extraOps ...string) {
	cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", testManifestName()))
	Expect(err).ToNot(HaveOccurred())
	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithDefaultCloudConfig()
	args := []string{
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"-v", "health_server_port=2345",
		"-o", "../test_yml_assets/enable-health-manifest-ops.yml",
		"--vars-store", "creds.yml",
		manifestPath,
	}

	args = append(args, extraOps...)

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, args...)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	allDeployedInstances = getInstanceInfos(boshBinaryPath)
}

func setupSecureGet() *httpclient.HTTPClient {
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/certificate",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	clientCertificate := stdOut

	stdOut, stdErr, exitStatus, err = cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/private_key",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	clientPrivateKey := stdOut

	stdOut, stdErr, exitStatus, err = cmdRunner.RunCommand(boshBinaryPath,
		"int", "creds.yml",
		"--path", "/dns_healthcheck_client_tls/ca",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	caCert := stdOut

	cert, err := tls.X509KeyPair([]byte(clientCertificate), []byte(clientPrivateKey))
	Expect(err).NotTo(HaveOccurred())

	logger := boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, ioutil.Discard)
	return healthclient.NewHealthClient([]byte(caCert), cert, logger)
}

func secureGetRespBody(client *httpclient.HTTPClient, hostname string, port int) ([]byte, error) {
	resp, err := secureGet(client, hostname, port)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func secureGet(client *httpclient.HTTPClient, hostname string, port int) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", hostname, port))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

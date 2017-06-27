package acceptance_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"net/http"
	"path/filepath"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
)

var _ = Describe("Integration", func() {
	var firstInstance instanceInfo

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDnsRelease()
			firstInstance = allDeployedInstances[0]
		})

		It("returns records for bosh instances", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A %s.dns.default.bosh-dns.bosh @%s", firstInstance.InstanceID, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Eventually(session.Out).Should(gbytes.Say(
				"%s\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s",
				firstInstance.InstanceID,
				firstInstance.IP))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns records for bosh instances found with query for all records", func() {
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-YWxs.dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-YWxs\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
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

		It("should resolve specified healthcheck", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A A healthcheck.bosh-dns. @%s", firstInstance.IP), " ")...)
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
			client, err := setupSecureGet(
				"../healthcheck/assets/test_certs/test_ca.pem",
				"../healthcheck/assets/test_certs/test_client.pem",
				"../healthcheck/assets/test_certs/test_client.key")
			Expect(err).ToNot(HaveOccurred())

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

func ensureHealthEndpointDeployed() {
	cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", testManifestName()))
	Expect(err).ToNot(HaveOccurred())
	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithDefaultCloudConfig()

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"--var-file", "health_ca=../healthcheck/assets/test_certs/test_ca.pem",
		"--var-file", "health_tls_cert=../healthcheck/assets/test_certs/test_server.pem",
		"--var-file", "health_tls_key=../healthcheck/assets/test_certs/test_server.key",
		"-v", "health_server_port=2345",
		"-o", "../healthcheck/assets/enable-health-manifest-ops.yml",
		manifestPath,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	allDeployedInstances = getInstanceInfos(boshBinaryPath)
}

func setupSecureGet(caFile, clientCertFile, clientKeyFile string) (*http.Client, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithIdentity(cert),
		tlsconfig.WithPivotalDefaults(),
	)

	clientConfig := tlsConfig.Client(tlsconfig.WithAuthority(caCertPool))
	clientConfig.BuildNameToCertificate()
	clientConfig.ServerName = "health.bosh-dns"

	transport := &http.Transport{TLSClientConfig: clientConfig}
	return &http.Client{Transport: transport}, nil
}

func secureGetRespBody(client *http.Client, hostname string, port int) ([]byte, error) {
	resp, err := secureGet(client, hostname, port)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func secureGet(client *http.Client, hostname string, port int) (*http.Response, error) {
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", hostname, port))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return resp, nil
}

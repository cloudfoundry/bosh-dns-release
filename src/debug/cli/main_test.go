package main_test

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/types"
)

func HaveTableRow(s ...string) types.GomegaMatcher {
	return gbytes.Say(`(?m)^\s*` + strings.Join(s, `\s+`) + `\s*$`)
}

var _ = Describe("Main", func() {
	var (
		server *ghttp.Server
	)

	BeforeEach(func() {
		server = newFakeAPIServer()
		GinkgoT().Setenv("DNS_API_ADDRESS", server.URL())
		GinkgoT().Setenv("DNS_API_TLS_CA_CERT_PATH", "../../bosh-dns/dns/api/assets/test_certs/test_ca.pem")
		GinkgoT().Setenv("DNS_API_TLS_CERTIFICATE_PATH", "../../bosh-dns/dns/api/assets/test_certs/test_wrong_cn_client.pem")
		GinkgoT().Setenv("DNS_API_TLS_PRIVATE_KEY_PATH", "../../bosh-dns/dns/api/assets/test_certs/test_client.key")
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("flags", func() {
		It("exits 1 if no argument is provided", func() {
			cmd := exec.Command(pathToCli)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
		})

		It("exits 1 if the command is not a valid command", func() {
			cmd := exec.Command(pathToCli, "explode")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("Unknown command"))
		})
	})

	Describe("instances", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/instances"),
					ghttp.RespondWith(http.StatusOK, `
							{
								"id":           "3",
								"group":        "1",
								"network":      "default",
								"deployment":   "dep",
								"ip":           "1.2.3.4",
								"domain":       "bosh",
								"az":           "z1",
								"index":        "0",
								"health_state": "healthy"
							}
							{
								"id":           "4",
								"group":        "2",
								"network":      "private",
								"deployment":   "dep-2",
								"ip":           "4.5.6.7",
								"domain":       "bosh",
								"az":           "z2",
								"index":        "1",
								"health_state": "unhealthy"
							}
						`),
				),
			)
		})

		It("renders the instances details", func() {
			cmd := exec.Command(pathToCli, "instances")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), string(session.Err.Contents()))
			Expect(session.Out).To(HaveTableRow("3", "1", "default", "dep", `1\.2\.3\.4`, "bosh", "z1", "0", "healthy"))
		})
	})

	Describe("local-groups", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/local-groups"),
					ghttp.RespondWith(http.StatusOK, `
							{
								"health_state": "running"
							}
							{
								"job_name": "zookeeper",
								"link_name": "conn",
								"link_type": "zookeeper",
								"group_id": "4",
								"health_state": "failing"
							}
							{
								"job_name": "zookeeper",
								"link_name": "peers",
								"link_type": "zookeeper_peers",
								"group_id": "5",
								"health_state": "running"
							}
							{
								"job_name": "consul",
								"link_name": "agent",
								"link_type": "conn",
								"group_id": "6"
							}
						`),
				),
			)
		})

		It("renders the local-groups details", func() {
			cmd := exec.Command(pathToCli, "local-groups")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), string(session.Err.Contents()))

			Expect(session.Out).To(HaveTableRow("JobName", "LinkName", "LinkType", "GroupID", "HealthState"))
			Expect(session.Out).To(HaveTableRow("-", "-", "-", "-", "running"))
			Expect(session.Out).To(HaveTableRow("zookeeper", "conn", "zookeeper", "4", "failing"))
			Expect(session.Out).To(HaveTableRow("zookeeper", "peers", "zookeeper_peers", "5", "running"))
			Expect(session.Out).To(HaveTableRow("consul", "agent", "conn", "6", "-"))
		})
	})
})

func newFakeAPIServer() *ghttp.Server {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithIdentityFromFile("../../bosh-dns/dns/api/assets/test_certs/test_server.pem", "../../bosh-dns/dns/api/assets/test_certs/test_server.key"),
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile("../../bosh-dns/dns/api/assets/test_certs/test_ca.pem"),
	)
	Expect(err).ToNot(HaveOccurred())
	tlsConfig.BuildNameToCertificate() //nolint:staticcheck

	server := ghttp.NewUnstartedServer()
	err = server.HTTPTestServer.Listener.Close()
	Expect(err).NotTo(HaveOccurred())

	suiteConfig, _ := GinkgoConfiguration()
	port := 2345 + suiteConfig.ParallelProcess
	server.HTTPTestServer.Listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	Expect(err).ToNot(HaveOccurred())

	server.HTTPTestServer.TLS = tlsConfig
	server.HTTPTestServer.StartTLS()

	return server
}

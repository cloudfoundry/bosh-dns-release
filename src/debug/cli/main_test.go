package main_test

import (
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Main", func() {
	Describe("flags", func() {
		It("exits 1 if no argument is provided", func() {
			cmd := exec.Command(pathToCli)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
		})

		It("exits 1 if the command is not `instances`", func() {
			cmd := exec.Command(pathToCli, "explode")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("Unknown command"))
		})
	})

	Describe("instances", func() {
		var (
			server *ghttp.Server
		)

		BeforeEach(func() {
			server = ghttp.NewServer()
			os.Setenv("DNS_DEBUG_API_ADDRESS", server.URL())
		})

		AfterEach(func() {
			server.Close()
			os.Unsetenv("DNS_DEBUG_API_ADDRESS")
		})

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
			Expect(session.Out).To(gbytes.Say(`3\s+1\s+default\s+dep\s+1\.2\.3\.4\s+bosh\s+z1\s+0\s+healthy`))
		})
	})
})

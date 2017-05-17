package no_recursors_configured

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"github.com/onsi/gomega/gexec"
	"time"
	"strings"
	"fmt"
)

var _ = Describe("dns job: recursors", func() {
	Describe("no recursors configured via job properties", func() {
		var (
			session       *gexec.Session
			firstInstance instanceInfo
		)
		BeforeEach(func() {
			var err error
			cmd := exec.Command(pathToTestRecursorServer, "53")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			session.Kill()
		})

		It("resolves the bosh-dns healthcheck", func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", "-d", boshDeployment, "dns/0", "-c", "dig +time=3 +tries=1 -t A healthcheck.bosh-dns."}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit())
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))

		})

		It("fowards queries to the configured recursors", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A example.com @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("example.com.\\s+0\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})
})

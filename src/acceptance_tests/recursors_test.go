package acceptance_test

import (
	"os/exec"
	"time"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("recursor", func() {
	var (
		session *gexec.Session
	)

	BeforeEach(func() {
		var err error
		cmd := exec.Command(pathToTestRecursorServer)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Kill()
	})

	It("returns success when receiving a truncated responses from a recursor", func() {
		By("ensuring the test recursor is returning truncated messages", func() {
			cmd := exec.Command("dig", strings.Split("+ignore +notcp -p 9955 -t A truncated-recursor.com. @127.0.0.1", " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		})

		By("ensuring the dns release returns a successful trucated recursed answer", func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +ignore +notcp -t A truncated-recursor.com. @169.254.0.2"}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		})
	})

	It("fowards queries to the configured recursors", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig -t A example.com @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		output := string(session.Out.Contents())
		Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Expect(output).To(MatchRegexp("example.com.\\s+0\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
		Expect(output).To(ContainSubstring("SERVER: 169.254.0.2#53"))
	})
})

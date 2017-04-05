package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Integration", func() {
	It("should start a dns server on port 53", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "sudo lsof -i :53"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("dns.*UDP 169.254.0.2:domain"))
		Eventually(session.Out).Should(gbytes.Say("dns.*TCP 169.254.0.2:domain"))
	})

	It("should respond to tcp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "dig +tcp bosh.io @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";bosh.io\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("should respond to udp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "dig +notcp bosh.io @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";bosh.io\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})
})

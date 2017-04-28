// +build linux

package linux_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"os/exec"
	"time"
)

var _ = Describe("Alias address binding", func() {
	It("should start a dns server on port 53", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo lsof -n -i :53"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		output := string(session.Out.Contents())
		Expect(output).To(MatchRegexp("dns.*TCP .*:domain"))
		Expect(output).To(MatchRegexp("dns.*UDP .*:domain"))
	})

	It("should respond to tcp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +tcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";healthcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("should respond to udp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +notcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";healthcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

})

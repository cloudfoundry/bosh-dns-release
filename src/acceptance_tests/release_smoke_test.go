package acceptance_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Integration", func() {
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

	It("returns records for bosh instances", func() {
		firstInstance := allDeployedInstances[0]

		cmd := exec.Command(
			boshBinaryPath,
			"ssh",
			firstInstanceSlug,
			"-c",
			fmt.Sprintf("dig -t A %s.dns.default.bosh-dns.bosh @169.254.0.2", firstInstance.InstanceID))
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(
			"%s\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s",
			firstInstance.InstanceID,
			firstInstance.IP))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("returns records for bosh instances found with query for all records", func() {
		Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

		cmd := exec.Command(boshBinaryPath, "ssh", firstInstanceSlug, "-c", "dig -t A q-YWxs.dns.default.bosh-dns.bosh @169.254.0.2")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		output := string(session.Out.Contents())
		Expect(output).To(ContainSubstring("Got answer:"))
		Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
		for _, info := range allDeployedInstances {
			Expect(output).To(MatchRegexp("q-YWxs\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
		}
		Expect(output).To(ContainSubstring("SERVER: 169.254.0.2#53"))
	})

	It("finds and resolves aliases specified in other jobs on the same instance", func() {
		cmd := exec.Command(boshBinaryPath, "ssh", "-c", "dig -t A internal.alias. @169.254.0.2")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))

		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})
})

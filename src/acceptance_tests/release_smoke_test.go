package acceptance_test

import (
	"fmt"
	"strings"

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
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "dig +tcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";healthcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("should respond to udp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "dig +notcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";healthcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("fowards queries to the configured recursors", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", "-c", "dig -t A pivotal.io @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 1"))
		Eventually(session.Out).Should(gbytes.Say("pivotal\\.io\\.\\s+\\d+\\s+IN\\s+A\\s+\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	PIt("returns records for bosh instances", func() {
		cmd := exec.Command(boshBinaryPath, []string{"instances", "--column", "Instance"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		instanceID := strings.Split(strings.TrimSpace(string(session.Out.Contents())), "/")[1]

		cmd = exec.Command(boshBinaryPath, []string{"instances", "--column", "IPs"}...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		ip := strings.TrimSpace(string(session.Out.Contents()))

		cmd = exec.Command(boshBinaryPath, []string{"ssh", "-c", fmt.Sprintf("dig -t A %s.dns.default.bosh-dns.bosh @169.254.0.2", instanceID)}...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say("%s\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", instanceID, ip))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})
})

//go:build windows
// +build windows

package windows_test

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("windows tests", func() {
	var localIP string

	BeforeEach(func() {
		var present bool
		localIP, present = os.LookupEnv("LOCAL_IP_ADDRESS")
		Expect(present).To(BeTrue(), "LOCAL_IP_ADDRESS environment variable not set")
		Expect(localIP).NotTo(BeEmpty(), "LOCAL_IP_ADDRESS environment variable not set")
	})

	It("should bind to tcp and udp", func() {
		cmd := exec.Command("powershell.exe", "-Command", "netstat -na | findstr :53")

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("TCP    %s:53", localIP)))
		Expect(session.Out.Contents()).To(ContainSubstring(fmt.Sprintf("UDP    %s:53", localIP)))
	})

	It("should respond to dns queries", func() {
		cmd := exec.Command("powershell.exe", "-Command", fmt.Sprintf("Resolve-DnsName -DnsOnly -Name upcheck.bosh-dns. -Server %s | Format-list", localIP))
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("IPAddress  : 127.0.0.1"))
		Expect(session.Out.Contents()).To(ContainSubstring("Name       : upcheck.bosh-dns"))
	})

	It("exposes a debug API through a CLI", func() {
		cmd := exec.Command("powershell.exe", "/var/vcap/jobs/bosh-dns-windows/bin/cli.ps1", "instances")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		Expect(session.Out).To(Say(`ID\s+Group\s+Network\s+Deployment\s+IP\s+Domain\s+AZ\s+Index\s+HealthState`))
		Expect(session.Out).To(Say(`[a-z0-9\-]{36}\s+acceptance-tests-windows\s+private\s+bosh-dns-windows-acceptance\s+[0-9.]+\s+bosh\.\s+z1\s+0\s+[a-z]+`))
	})

	Context("as the system-configured nameserver", func() {
		It("should respond to dns queries", func() {
			cmd := exec.Command("powershell.exe", "-Command", "Resolve-DnsName -DnsOnly -Name upcheck.bosh-dns. | Format-list")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("IPAddress  : 127.0.0.1"))
			Expect(session.Out.Contents()).To(ContainSubstring("Name       : upcheck.bosh-dns"))
		})
	})

	It("sets the DNS cache service negative cache TTL to 0", func() {
		cmd := exec.Command("powershell.exe", "$Value = Get-ItemProperty -Path HKLM:\\SYSTEM\\CurrentControlSet\\Services\\Dnscache\\Parameters; $Value.MaxNegativeCacheTtl")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		Expect(session.Out).To(Say("0"))
	})

	It("runs the bosh-dns process with High priority", func() {
		cmd := exec.Command("powershell.exe", "-command", "(Get-Process -name bosh-dns).PriorityClass")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		Expect(session.Out).To(Say("High"))
	})

	It("sets the DNS cache service server priority time limit to 0", func() {
		cmd := exec.Command("powershell.exe", "$Value = Get-ItemProperty -Path HKLM:\\SYSTEM\\CurrentControlSet\\Services\\Dnscache\\Parameters; $Value.ServerPriorityTimeLimit")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		Expect(session.Out).To(Say("0"))
	})
})

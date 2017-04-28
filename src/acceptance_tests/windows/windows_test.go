// +build windows
package windows_test

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("windows tests", func() {
	It("should bind to tcp and udp", func() {
		cmd := exec.Command("powershell.exe", "assets/netstat.ps1")

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("TCP    169.254.0.2:53"))
		Expect(session.Out.Contents()).To(ContainSubstring("UDP    169.254.0.2:53"))
	})

	It("should respond to dns queries", func() {
		cmd := exec.Command("powershell.exe", "assets/resolve-dnsname.ps1")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("Address        : 127.0.0.1"))
		Expect(session.Out.Contents()).To(ContainSubstring("Name           : healthcheck.bosh-dns"))
	})
})

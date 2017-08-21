// +build windows

package disable_nameserver_override_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"strings"
	"time"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("dns job: disable_nameserver_override", func() {
	Context("when the system has not been configured to use the bosh dns server", func() {
		It("does not resolve the bosh-dns upcheck", func() {
			cmd := exec.Command("powershell.exe", "-Command", "Resolve-DnsName -DnsOnly -Type A -Name upcheck.bosh-dns.")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit())
			output := string(session.Err.Contents())
			Expect(output).To(ContainSubstring("DNS_ERROR_RCODE_NAME_ERROR"))
		})

		Context("external processes changing dns servers", func() {
			var existingDNS string

			BeforeEach(func() {
				cmd := exec.Command("powershell.exe", "/var/vcap/packages/bosh-dns-windows/bin/list-server-addresses.ps1")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))

				existingDNS = strings.SplitN(string(session.Out.Contents()), "\r\n", 2)[0]
			})

			AfterEach(func() {
				cmd := exec.Command("powershell.exe", "/var/vcap/packages/bosh-dns-windows/bin/prepend-dns-server.ps1", "-DNSAddress", existingDNS)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			})

			It("does not rewrite the nameserver configuration to our dns server", func() {
				cmd := exec.Command("powershell.exe", "/var/vcap/packages/bosh-dns-windows/bin/prepend-dns-server.ps1", "-DNSAddress", "192.0.2.100")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))

				Consistently(func() string {
					cmd := exec.Command("powershell.exe", "/var/vcap/packages/bosh-dns-windows/bin/list-server-addresses.ps1")
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session, 10*time.Second).Should(gexec.Exit(0))

					return strings.SplitN(string(session.Out.Contents()), "\r\n", 2)[0]
				}, 15*time.Second, time.Second*2).Should(Equal("192.0.2.100"))
			})
		})
	})
})

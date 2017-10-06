// +build windows

package disable_nameserver_override_test

import (
	"os/exec"
	"time"

	"bosh-dns/dns/manager"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			var (
				existingDNS string
				man         manager.DNSManager
			)

			BeforeEach(func() {
				logger := logger.NewLogger(logger.LevelDebug)
				cmdRunner := system.NewExecCmdRunner(logger)
				fs := system.NewOsFileSystem(logger)
				man = manager.NewWindowsManager(cmdRunner, fs)

				addresses, err := man.Read()
				Expect(err).NotTo(HaveOccurred())
				existingDNS = addresses[0]
			})

			AfterEach(func() {
				err := man.SetPrimary(existingDNS)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not rewrite the nameserver configuration to our dns server", func() {
				err := man.SetPrimary("192.0.2.100")
				Expect(err).NotTo(HaveOccurred())

				Consistently(func() string {
					addresses, err := man.Read()
					Expect(err).NotTo(HaveOccurred())

					return addresses[0]
				}, 15*time.Second, time.Second*2).Should(Equal("192.0.2.100"))
			})
		})
	})
})

package manager_test

import (
	"errors"
	"fmt"

	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	"bosh-dns/dns/manager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsManager", func() {
	var (
		dnsManager    manager.DNSManager
		fakeCmdRunner *boshsysfakes.FakeCmdRunner
		address       = "192.0.2.100"
	)

	BeforeEach(func() {
		fakeCmdRunner = boshsysfakes.NewFakeCmdRunner()
		dnsManager = manager.NewWindowsManager(fakeCmdRunner)
	})

	Describe("Read", func() {
		Context("powershell fails", func() {
			It("errors for list", func() {
				fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

				_, err := dnsManager.Read()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing list-server-addresses.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		It("splits lines and returns as slice", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: fmt.Sprintf("%s\r\n%s", "8.8.8.8", address)})

			servers, err := dnsManager.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(servers).To(ConsistOf("8.8.8.8", address))
		})

		It("returns an empty array when no servers are configured", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: ""})

			servers, err := dnsManager.Read()

			Expect(err).ToNot(HaveOccurred())
			Expect(servers).To(HaveLen(0))
		})
	})

	Describe("SetPrimary", func() {

		Context("powershell fails", func() {
			It("errors for prepend", func() {
				fakeCmdRunner.AddCmdResult(fmt.Sprintf("powershell.exe /var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1 %s", address), boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing prepend-dns-server.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})

			It("errors for list", func() {
				fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing list-server-addresses.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		It("can execute powershell successfully", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: fmt.Sprintf("%s\r\n%s", "8.8.8.8", address)})
			fakeCmdRunner.AddCmdResult(fmt.Sprintf("powershell.exe /var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1 %s", address), boshsysfakes.FakeCmdResult{})

			err := dnsManager.SetPrimary(address)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(HaveLen(2))
			Expect(fakeCmdRunner.RunCommands).To(ConsistOf(
				[]string{"powershell.exe", "/var/vcap/packages/dns-windows/bin/list-server-addresses.ps1"},
				[]string{"powershell.exe", "/var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1", address},
			))
		})

		It("skips if dns is already configured", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: fmt.Sprintf("%s\r\n", address)})

			err := dnsManager.SetPrimary(address)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCmdRunner.RunCommands).To(HaveLen(1))
			Expect(fakeCmdRunner.RunCommands).To(ConsistOf([][]string{{"powershell.exe", "/var/vcap/packages/dns-windows/bin/list-server-addresses.ps1"}}))
		})
	})
})

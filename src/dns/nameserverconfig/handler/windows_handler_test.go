package handler_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/nameserverconfig/handler"

	"errors"
	"fmt"
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolvConfCheck", func() {
	var (
		handler          WindowsHandler
		fakeCmdRunner    *boshsysfakes.FakeCmdRunner
		correctAddress   string = "192.0.2.100"
		incorrectAddress string = "192.0.2.222"
	)

	BeforeEach(func() {
		fakeCmdRunner = boshsysfakes.NewFakeCmdRunner()
		handler = NewWindowsHandler(correctAddress, fakeCmdRunner)
	})

	Describe("Apply", func() {
		Context("powershell fails", func() {
			It("errors", func() {
				fakeCmdRunner.AddCmdResult(fmt.Sprintf("powershell.exe /var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1 %s", correctAddress), boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

				err := handler.Apply()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing prepend-dns-server.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		It("can execute powershell successfully", func() {
			fakeCmdRunner.AddCmdResult(fmt.Sprintf("powershell.exe /var/vcap/packages/dns-windows/bin/prepend-dns-server.ps1 %s", correctAddress), boshsysfakes.FakeCmdResult{})

			err := handler.Apply()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("IsCorrect", func() {
		It("errors when powershell fails", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

			_, err := handler.IsCorrect()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Executing list-server-addresses.ps1"))
			Expect(err.Error()).To(ContainSubstring("fake-err1"))
		})

		It("detects when settings are valid", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: fmt.Sprintf("%s\r\n", correctAddress)})

			res, err := handler.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(true))
		})

		It("detects when our address is not the first server", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{Stdout: fmt.Sprintf("%s\r\n%s", incorrectAddress, correctAddress)})

			res, err := handler.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(false))
		})

		It("returns false if there are no servers", func() {
			fakeCmdRunner.AddCmdResult("powershell.exe /var/vcap/packages/dns-windows/bin/list-server-addresses.ps1", boshsysfakes.FakeCmdResult{})

			res, err := handler.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(false))
		})

	})
})

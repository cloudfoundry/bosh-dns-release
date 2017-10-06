package manager_test

import (
	"errors"
	"fmt"
	"path/filepath"

	"bosh-dns/dns/manager"
	"bosh-dns/dns/manager/managerfakes"

	systemfakes "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsManager", func() {
	var (
		dnsManager     manager.DNSManager
		fakeCmdRunner  *managerfakes.FakeCmdRunner
		fakeFileSystem *systemfakes.FakeFileSystem
		address        = "192.0.2.100"
	)

	BeforeEach(func() {
		fakeCmdRunner = &managerfakes.FakeCmdRunner{}
		fakeFileSystem = systemfakes.NewFakeFileSystem()
		dnsManager = manager.NewWindowsManager(fakeCmdRunner, fakeFileSystem)
	})

	Describe("Read", func() {
		Context("powershell fails", func() {
			It("errors for list", func() {
				fakeCmdRunner.RunCommandReturns("", "", 1, errors.New("fake-err1"))

				_, err := dnsManager.Read()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing list-server-addresses.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		It("splits lines and returns as slice", func() {
			fakeCmdRunner.RunCommandReturns(fmt.Sprintf("%s\r\n%s", "8.8.8.8", address), "", 0, nil)

			servers, err := dnsManager.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(servers).To(ConsistOf("8.8.8.8", address))
		})

		It("returns an empty array when no servers are configured", func() {
			fakeCmdRunner.RunCommandReturns("", "", 0, nil)

			servers, err := dnsManager.Read()

			Expect(err).ToNot(HaveOccurred())
			Expect(servers).To(HaveLen(0))
		})

		Context("when an error occurs", func() {
			Context("when the temp dir cannot be created", func() {
				It("returns an error", func() {
					fakeFileSystem.TempDirError = errors.New("no temp dir")

					_, err := dnsManager.Read()
					Expect(err).To(MatchError("Creating list-server-addresses.ps1: no temp dir"))
				})
			})

			Context("when the file cannot be written", func() {
				It("returns an error", func() {
					fakeFileSystem.WriteFileError = errors.New("no file written")

					_, err := dnsManager.Read()
					Expect(err).To(MatchError("Creating list-server-addresses.ps1: no file written"))
				})
			})

			Context("when the file cannot be chmod'ed", func() {
				It("returns an error", func() {
					fakeFileSystem.ChmodErr = errors.New("no chmodding allowed")

					_, err := dnsManager.Read()
					Expect(err).To(MatchError("Creating list-server-addresses.ps1: no chmodding allowed"))
				})
			})
		})
	})

	Describe("SetPrimary", func() {
		Context("powershell fails", func() {
			It("errors for prepend", func() {
				fakeCmdRunner.RunCommandReturnsOnCall(1, "", "", 1, errors.New("fake-err1"))

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing prepend-dns-server.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})

			It("errors for list", func() {
				fakeCmdRunner.RunCommandReturns("", "", 1, errors.New("fake-err1"))

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing list-server-addresses.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		It("can execute powershell successfully", func() {
			paths := []string{}
			fakeCmdRunner.RunCommandStub = func(cmdName string, args ...string) (string, string, int, error) {
				Expect(cmdName).To(Equal("powershell.exe"))
				Expect(args[0]).To(MatchRegexp(`.ps1$`))
				paths = append(paths, args[0])

				stats, err := fakeFileSystem.Stat(args[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Size()).To(BeNumerically(">", 0))

				Expect(err).NotTo(HaveOccurred())
				return fmt.Sprintf("%s\r\n%s", "8.8.8.8", address), "", 0, nil
			}

			err := dnsManager.SetPrimary(address)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCmdRunner.RunCommandCallCount()).To(Equal(2))

			for _, path := range paths {
				Expect(fakeFileSystem.FileExists(filepath.Dir(path))).To(BeFalse())
			}
		})

		It("skips if dns is already configured", func() {
			fakeCmdRunner.RunCommandReturns(fmt.Sprintf("%s\r\n", address), "", 0, nil)

			err := dnsManager.SetPrimary(address)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCmdRunner.RunCommandCallCount()).To(Equal(1))
		})
	})
})

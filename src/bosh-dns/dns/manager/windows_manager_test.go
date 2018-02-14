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
		dnsManager         manager.DNSManager
		fakeAdapterFetcher *managerfakes.FakeAdapterFetcher
		fakeCmdRunner      *managerfakes.FakeCmdRunner
		fakeFileSystem     *systemfakes.FakeFileSystem
		address            = "192.0.2.100"
	)

	const (
		NotLoopBack uint32 = 23
		NotTunnel   uint32 = 130
		NonUp       uint32 = 0
	)

	BeforeEach(func() {
		fakeCmdRunner = &managerfakes.FakeCmdRunner{}
		fakeFileSystem = systemfakes.NewFakeFileSystem()
		fakeAdapterFetcher = &managerfakes.FakeAdapterFetcher{}
		dnsManager = manager.NewWindowsManager(fakeCmdRunner, fakeFileSystem, fakeAdapterFetcher)
	})

	Describe("Read", func() {
		Context("when an error occurs", func() {
			It("returns an error", func() {
				fakeAdapterFetcher.AdaptersReturns(nil, errors.New("Failed to fetch adapters"))

				_, err := dnsManager.Read()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to fetch adapters"))
				Expect(err.Error()).To(ContainSubstring("Getting list of current DNS Servers"))
			})
		})

		Context("when adapters are found", func() {
			It("filters out loopback adapters", func() {
				fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
					{
						IfType:             manager.IfTypeSoftwareLoopback,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "C1:F2:2A:44:73:5E",
						DNSServerAddresses: []string{"8.8.8.8", "8.8.4.4"},
					},
					{
						IfType:             NotLoopBack,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "A1:F2:2A:44:73:5F",
						DNSServerAddresses: []string{"1.1.1.1"},
					},
				}, nil)

				servers, err := dnsManager.Read()
				Expect(err).ToNot(HaveOccurred())
				Expect(servers).To(ConsistOf("1.1.1.1"))
			})

			It("filters out tunnel adapters", func() {
				fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
					{
						IfType:             manager.IfTypeTunnel,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "C1:F2:2A:44:73:5E",
						DNSServerAddresses: []string{"8.8.8.8", "8.8.4.4"},
					},
					{
						IfType:             NotTunnel,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "A1:F2:2A:44:73:5F",
						DNSServerAddresses: []string{"1.1.1.1"},
					},
				}, nil)

				servers, err := dnsManager.Read()
				Expect(err).ToNot(HaveOccurred())
				Expect(servers).To(ConsistOf("1.1.1.1"))
			})

			It("filters out non-physical adapters", func() {
				fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
					{
						IfType:             NotLoopBack,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "C1:F2:2A:44:73:5E",
						DNSServerAddresses: []string{"8.8.8.8", "8.8.4.4"},
					},
					{
						IfType:             NotTunnel,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "00:03:FF:44:73:5F",
						DNSServerAddresses: []string{"1.1.1.1"},
					},
				}, nil)

				servers, err := dnsManager.Read()
				Expect(err).ToNot(HaveOccurred())
				Expect(servers).To(ConsistOf("8.8.8.8", "8.8.4.4"))
			})

			It("filter out non-up adapters", func() {
				fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
					{
						IfType:             NotLoopBack,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "C1:F2:2A:44:73:5E",
						DNSServerAddresses: []string{"8.8.8.8", "8.8.4.4"},
					},
					{
						IfType:             NotTunnel,
						OperStatus:         NonUp,
						PhysicalAddress:    "A1:F2:2A:44:73:5F",
						DNSServerAddresses: []string{"1.1.1.1"},
					},
				}, nil)

				servers, err := dnsManager.Read()
				Expect(err).ToNot(HaveOccurred())
				Expect(servers).To(ConsistOf("8.8.8.8", "8.8.4.4"))
			})
		})

		It("returns an empty array when no servers are configured", func() {
			fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{}, nil)

			servers, err := dnsManager.Read()
			Expect(err).ToNot(HaveOccurred())
			Expect(servers).To(HaveLen(0))
		})
	})

	Describe("SetPrimary", func() {
		Context("powershell fails", func() {
			It("errors for prepend", func() {
				fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
					{
						IfType:             NotLoopBack,
						OperStatus:         manager.IfOperStatusUp,
						PhysicalAddress:    "A1:F2:2A:44:73:5F",
						DNSServerAddresses: []string{},
					},
				}, nil)
				fakeCmdRunner.RunCommandReturns("", "", 1, errors.New("fake-err1"))

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing prepend-dns-server.ps1"))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})

			It("errors for list", func() {
				fakeAdapterFetcher.AdaptersReturns(nil, errors.New("Failed to fetch adapters"))

				err := dnsManager.SetPrimary(address)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to fetch adapters"))
				Expect(err.Error()).To(ContainSubstring("Getting list of current DNS Servers"))
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

			Expect(fakeCmdRunner.RunCommandCallCount()).To(Equal(1))

			for _, path := range paths {
				Expect(fakeFileSystem.FileExists(filepath.Dir(path))).To(BeFalse())
			}
		})

		It("skips if dns is already configured", func() {
			fakeCmdRunner.RunCommandReturns(fmt.Sprintf("%s\r\n", address), "", 0, nil)
			fakeAdapterFetcher.AdaptersReturns([]manager.Adapter{
				{
					IfType:             NotLoopBack,
					OperStatus:         manager.IfOperStatusUp,
					PhysicalAddress:    "A1:F2:2A:44:73:5F",
					DNSServerAddresses: []string{address, "1.1.1.1"},
				},
			}, nil)

			err := dnsManager.SetPrimary(address)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCmdRunner.RunCommandCallCount()).To(Equal(0))
		})
	})
})

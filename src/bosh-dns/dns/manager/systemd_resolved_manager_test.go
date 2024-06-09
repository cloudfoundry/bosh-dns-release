package manager_test

import (
	"bosh-dns/dns/manager"

	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemdResolvedManager", func() {
	var (
		fakeCmdRunner *fakes.FakeCmdRunner
	)

	BeforeEach(func() {
		fakeCmdRunner = fakes.NewFakeCmdRunner()
	})

	Describe("UpdateDomains", func() {
		It("configures the Domains for the bosh-dns interface using resolvectl", func() {
			fakeCmdRunner.AddCmdResult("resolvectl domain bosh-dns bosh. alias-domain.", fakes.FakeCmdResult{})

			manager := manager.NewSystemdResolvedManager(fakeCmdRunner)

			err := manager.UpdateDomains([]string{"bosh.", "alias-domain."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[0]).To(Equal([]string{"resolvectl", "domain", "bosh-dns", "bosh.", "alias-domain."}))
		})
	})
})

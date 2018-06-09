package aliases_test

import (
	. "bosh-dns/dns/server/aliases"

	"fmt"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AliasesProvider", func() {
	var (
		fakeFS          *boshsysfakes.FakeFileSystem
		aliasesProvider AliasesProvider
		fakeLogger      *fakes.FakeLogger
		testFile        string
		pattern         string
		domainToVerify  string
	)

	BeforeEach(func() {
		testFile = "/xiaolongxia.aliases"
		fakeFS = boshsysfakes.NewFakeFileSystem()
		domainToVerify = "master.cfcr.internal"
		fakeLogger = &fakes.FakeLogger{}
		aliasesProvider = NewAutoRefreshAliasesProvider(fakeLogger, pattern, fakeFS, time.Second)

	})

	Describe("AliasesProvider", func() {
		Context("Added", func() {
			It("added alias file", func() {
				fakeFS.SetGlob(pattern, []string{testFile})
				fakeFS.WriteFile(testFile, []byte(fmt.Sprintf(`{
					"%s":["*.master.default.my-cluster.bosh"],
					"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
					}`, domainToVerify)))
				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					fmt.Fprintln(GinkgoWriter, hosts)
					for _, host := range hosts {
						if host == fmt.Sprintf("%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeTrue())
			})
		})

		Context("Updated", func() {
			It("updated alias file", func() {
				fakeFS.SetGlob(pattern, []string{testFile})
				fakeFS.WriteFile(testFile, []byte(fmt.Sprintf(`{
						"%s":["*.master.default.my-cluster.bosh"],
						"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
						}`, domainToVerify)))

				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					for _, host := range hosts {
						if host == fmt.Sprintf("%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeTrue())

				fakeFS.WriteFile(testFile, []byte(fmt.Sprintf(`{
						"%s":["*.master.default.my-cluster.bosh"],
						"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
						}`, fmt.Sprintf("new-%s", domainToVerify))))

				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					for _, host := range hosts {
						if host == fmt.Sprintf("%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeFalse())

				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					for _, host := range hosts {
						if host == fmt.Sprintf("new-%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeTrue())

			})
		})

		Context("Deleted", func() {
			It("deleted alias file", func() {
				fakeFS.SetGlob(pattern, []string{testFile})
				fakeFS.WriteFile(testFile, []byte(fmt.Sprintf(`{
						"%s":["*.master.default.my-cluster.bosh"],
						"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
						}`, domainToVerify)))

				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					for _, host := range hosts {
						if host == fmt.Sprintf("%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeTrue())

				fakeFS.SetGlob(pattern, []string{})

				Eventually(func() bool {
					hosts := aliasesProvider.AliasHosts()
					fmt.Fprintln(GinkgoWriter, hosts)
					for _, host := range hosts {
						if host == fmt.Sprintf("%s.", domainToVerify) {
							return true
						}
					}
					return false
				}, time.Second*5, time.Second).Should(BeFalse())

				Eventually(func() int {
					hosts := aliasesProvider.AliasHosts()
					return len(hosts)
				}, time.Second*5, time.Second).Should(Equal(0))
			})
		})
	})
})

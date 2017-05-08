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
		resolvConfCheck  ResolvConfHandler
		fakeFileSystem   *boshsysfakes.FakeFileSystem
		fakeCmdRunner    *boshsysfakes.FakeCmdRunner
		correctAddress   string = "192.0.2.100"
		incorrectAddress string = "192.0.2.222"
	)

	BeforeEach(func() {
		fakeFileSystem = boshsysfakes.NewFakeFileSystem()
		fakeCmdRunner = boshsysfakes.NewFakeCmdRunner()
		resolvConfCheck = NewResolvConfHandler(correctAddress, fakeFileSystem, fakeCmdRunner)
	})

	Describe("Apply", func() {
		Context("filesystem fails", func() {
			It("errors", func() {
				fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{})
				fakeFileSystem.WriteFileError = errors.New("fake-err1")

				err := resolvConfCheck.Apply()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Writing "))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		Context("resolvconf update fails", func() {
			It("errors", func() {
				fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{ExitStatus: 1, Error: errors.New("fake-err1")})

				err := resolvConfCheck.Apply()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Executing "))
				Expect(err.Error()).To(ContainSubstring("fake-err1"))
			})
		})

		Context("resolvconf fails to rewrite /etc/resolv.conf", func() {
			It("errors if resolvconf update fails", func() {
				fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{})

				err := resolvConfCheck.Apply()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to confirm nameserver "))
			})
		})

		It("creates /etc/resolvconf/resolv.conf.d/head with our DNS server", func() {
			fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{})

			// theoretically `resolvconf -u` rewrites this file externally
			fakeFileSystem.WriteFileString("/etc/resolv.conf", `nameserver 192.0.2.100`)

			err := resolvConfCheck.Apply()
			Expect(err).NotTo(HaveOccurred())

			contents, err := fakeFileSystem.ReadFileString("/etc/resolvconf/resolv.conf.d/head")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal(`# This file was automatically updated by bosh-dns
nameserver 192.0.2.100
`))
		})

		It("avoids prepending itself more than once (in case resolvconf is slower than our check interval)", func() {
			fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{})

			// theoretically `resolvconf -u` rewrites this file externally
			fakeFileSystem.WriteFileString("/etc/resolv.conf", `nameserver 192.0.2.100`)

			err := fakeFileSystem.WriteFileString("/etc/resolvconf/resolv.conf.d/head", `
nameserver 192.0.2.100
nameserver 8.8.8.8
`)
			Expect(err).NotTo(HaveOccurred())

			err = resolvConfCheck.Apply()
			Expect(err).NotTo(HaveOccurred())

			contents, err := fakeFileSystem.ReadFileString("/etc/resolvconf/resolv.conf.d/head")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal(`# This file was automatically updated by bosh-dns
nameserver 192.0.2.100
`))
		})

		It("prepends /etc/resolvconf/resolv.conf.d/head with our DNS server", func() {
			fakeFileSystem.WriteFileString("/etc/resolvconf/resolv.conf.d/head", `# some comment
nameserver 192.0.3.1
nameserver 192.0.3.2
`)

			// theoretically `resolvconf -u` rewrites this file externally
			fakeFileSystem.WriteFileString("/etc/resolv.conf", `nameserver 192.0.2.100`)

			err := resolvConfCheck.Apply()
			Expect(err).NotTo(HaveOccurred())

			contents, err := fakeFileSystem.ReadFileString("/etc/resolvconf/resolv.conf.d/head")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal(`# This file was automatically updated by bosh-dns
nameserver 192.0.2.100

# some comment
nameserver 192.0.3.1
nameserver 192.0.3.2
`))
		})
	})

	Describe("IsCorrect", func() {
		BeforeEach(func() {
			err := fakeFileSystem.WriteFileString("/etc/resolv.conf", fmt.Sprintf(`
nameserver %s
`, correctAddress))
			Expect(err).NotTo(HaveOccurred())

			fakeCmdRunner.AddCmdResult("resolvconf -u", boshsysfakes.FakeCmdResult{})
		})

		It("errors when resolv.conf cannot be read", func() {
			fakeFileSystem.ReadFileError = errors.New("fake-err1")

			_, err := resolvConfCheck.IsCorrect()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Reading "))
			Expect(err.Error()).To(ContainSubstring("fake-err1"))
		})

		It("detects when resolv.conf is invalid", func() {
			resolvConfCheck = NewResolvConfHandler(incorrectAddress, fakeFileSystem, fakeCmdRunner)

			res, err := resolvConfCheck.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(false))
		})

		It("detects when resolv.conf is valid", func() {
			res, err := resolvConfCheck.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(true))
		})

		It("detects when resolv.conf has our DNS address as the first entry", func() {
			err := fakeFileSystem.WriteFileString("/etc/resolv.conf", fmt.Sprintf(`
nameserver %s
nameserver %s
`, incorrectAddress, correctAddress))

			Expect(err).NotTo(HaveOccurred())

			res, err := resolvConfCheck.IsCorrect()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(false))
		})
	})
})

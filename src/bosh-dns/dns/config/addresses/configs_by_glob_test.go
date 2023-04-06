package addresses_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/config/addresses"
	"bosh-dns/dns/config/addresses/addressesfakes"
)

var _ = Describe("ConfigFromGlob", func() {
	var (
		fakeGlobber *addressesfakes.FakeConfigGlobber
		fakeLoader  *addressesfakes.FakeNamedConfigLoader
	)

	BeforeEach(func() {
		fakeGlobber = &addressesfakes.FakeConfigGlobber{}
		fakeLoader = &addressesfakes.FakeNamedConfigLoader{}
	})

	It("queries the globber", func() {
		_, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
		Expect(err).ToNot(HaveOccurred())

		Expect(fakeGlobber.GlobCallCount()).To(Equal(1))
		Expect(fakeGlobber.GlobArgsForCall(0)).To(Equal("someglob"))
	})

	Context("when the globber fails to glob", func() {
		BeforeEach(func() {
			fakeGlobber.GlobReturns(nil, errors.New("glob-you-dont"))
		})
		It("promotes the error", func() {
			_, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("glob pattern failed to compute"))
			Expect(err.Error()).To(ContainSubstring("glob-you-dont"))
		})
	})

	Context("when the globber finds some files with the glob", func() {
		BeforeEach(func() {
			fakeGlobber.GlobReturns([]string{"/some/file", "/another/file"}, nil)
		})

		It("tries to load the configs by name", func() {
			_, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeLoader.LoadCallCount()).To(Equal(2))
			Expect(fakeLoader.LoadArgsForCall(0)).To(Equal("/some/file"))
			Expect(fakeLoader.LoadArgsForCall(1)).To(Equal("/another/file"))
		})

		Context("when the loader is unable to load the config", func() {
			BeforeEach(func() {
				fakeLoader.LoadReturns(nil, errors.New("file-is-busted"))
			})
			It("promotes the error", func() {
				_, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not load config"))
				Expect(err.Error()).To(ContainSubstring("file-is-busted"))
			})
		})

		Context("when the loader loads one or more configs", func() {
			BeforeEach(func() {
				fakeLoader.LoadStub = func(name string) (AddressConfigs, error) {
					switch name {
					case "/some/file":
						return AddressConfigs{
							{
								Address: "10.0.14.4",
								Port:    53,
							},
							{
								Address: "172.13.3.5",
								Port:    51,
							},
						}, nil
					case "/another/file":
						return AddressConfigs{
							{
								Address: "101.0.14.4",
								Port:    63,
							},
						}, nil
					}
					return nil, errors.New("wrong-name")
				}
			})

			It("appends all addresses", func() {
				c, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
				Expect(err).ToNot(HaveOccurred())
				Expect(c).To(Equal(AddressConfigs{
					{
						Address: "10.0.14.4",
						Port:    53,
					},
					{
						Address: "172.13.3.5",
						Port:    51,
					},
					{
						Address: "101.0.14.4",
						Port:    63,
					},
				}))
			})
		})
	})
})

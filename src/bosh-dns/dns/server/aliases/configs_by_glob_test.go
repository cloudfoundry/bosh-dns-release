package aliases_test

import (
	. "bosh-dns/dns/server/aliases"

	"bosh-dns/dns/server/aliases/aliasesfakes"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigFromGlob", func() {
	var (
		fakeGlobber *aliasesfakes.FakeConfigGlobber
		fakeLoader  *aliasesfakes.FakeNamedConfigLoader
	)

	BeforeEach(func() {
		fakeGlobber = &aliasesfakes.FakeConfigGlobber{}
		fakeLoader = &aliasesfakes.FakeNamedConfigLoader{}
	})

	It("queries the globber", func() {
		ConfigFromGlob(fakeGlobber, fakeLoader, "someglob") //nolint:errcheck
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
			ConfigFromGlob(fakeGlobber, fakeLoader, "someglob") //nolint:errcheck
			Expect(fakeLoader.LoadCallCount()).To(Equal(2))
			Expect(fakeLoader.LoadArgsForCall(0)).To(Equal("/some/file"))
			Expect(fakeLoader.LoadArgsForCall(1)).To(Equal("/another/file"))
		})

		Context("when the loader is unable to load the config", func() {
			BeforeEach(func() {
				fakeLoader.LoadReturns(Config{}, errors.New("file-is-busted"))
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
				fakeLoader.LoadStub = func(name string) (Config, error) {
					switch name {
					case "/some/file":
						return MustNewConfigFromMap(map[string][]string{
							"alias1": {"alias2"},
						}), nil
					case "/another/file":
						return MustNewConfigFromMap(map[string][]string{
							"alias2": {"domain2"},
						}), nil
					}
					return Config{}, errors.New("wrong-name")
				}
			})

			It("merges and reduces the files", func() {
				c, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
				Expect(err).ToNot(HaveOccurred())
				Expect(c).To(Equal(MustNewConfigFromMap(map[string][]string{
					"alias1": {"domain2"},
					"alias2": {"domain2"},
				})))
			})

			Context("when the reduction fails due to cyclic aliases", func() {
				BeforeEach(func() {
					fakeLoader.LoadStub = func(name string) (Config, error) {
						switch name {
						case "/some/file":
							return MustNewConfigFromMap(map[string][]string{
								"alias1": {"alias2"},
							}), nil
						case "/another/file":
							return MustNewConfigFromMap(map[string][]string{
								"alias2": {"alias1"},
							}), nil
						}
						return Config{}, errors.New("wrong-name")
					}
				})

				It("promotes the error", func() {
					_, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("could not produce valid alias config"))
					Expect(err.Error()).To(ContainSubstring("recursion detected"))
				})
			})
		})
	})
})

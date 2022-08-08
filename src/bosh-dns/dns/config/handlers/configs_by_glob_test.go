package handlers_test

import (
	"bosh-dns/dns/config"
	. "bosh-dns/dns/config/handlers"
	"bosh-dns/dns/config/handlers/handlersfakes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigFromGlob", func() {
	var (
		fakeGlobber *handlersfakes.FakeConfigGlobber
		fakeLoader  *handlersfakes.FakeNamedConfigLoader
	)

	BeforeEach(func() {
		fakeGlobber = &handlersfakes.FakeConfigGlobber{}
		fakeLoader = &handlersfakes.FakeNamedConfigLoader{}
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
				fakeLoader.LoadStub = func(name string) (HandlerConfigs, error) {
					switch name {
					case "/some/file":
						return HandlerConfigs{
							{
								Domain: "local.internal.",
								Cache:  config.Cache{Enabled: true},
								Source: Source{Type: "http", URL: "http://some.endpoint.local"},
							},
							{
								Domain: "local2.internal.",
								Cache:  config.Cache{Enabled: true},
								Source: Source{Type: "http", URL: "http://some2.endpoint.local"},
							},
						}, nil
					case "/another/file":
						return HandlerConfigs{
							{
								Domain: "local.internal2.",
								Cache:  config.Cache{Enabled: false},
								Source: Source{Type: "dns", Recursors: []string{"127.0.0.1:53"}},
							},
						}, nil
					}
					return nil, errors.New("wrong-name")
				}
			})

			It("appends all handlers", func() {
				c, err := ConfigFromGlob(fakeGlobber, fakeLoader, "someglob")
				Expect(err).ToNot(HaveOccurred())
				Expect(c).To(Equal(HandlerConfigs{
					{
						Domain: "local.internal.",
						Cache:  config.Cache{Enabled: true},
						Source: Source{Type: "http", URL: "http://some.endpoint.local"},
					},
					{
						Domain: "local2.internal.",
						Cache:  config.Cache{Enabled: true},
						Source: Source{Type: "http", URL: "http://some2.endpoint.local"},
					},
					{
						Domain: "local.internal2.",
						Cache:  config.Cache{Enabled: false},
						Source: Source{Type: "dns", Recursors: []string{"127.0.0.1:53"}},
					},
				}))
			})
		})
	})
})

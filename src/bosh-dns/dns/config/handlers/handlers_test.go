package handlers_test

import (
	. "bosh-dns/dns/config/handlers"
	. "bosh-dns/dns/config/handlers/handlersfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handlers Configuration", func() {
	Describe("GenerateHandlers", func() {
		var (
			handlersConfig     HandlersConfig
			fakeHandlerFactory *FakeHandlerFactory
			fakeJsonHandler    *FakeDnsHandler
			fakeDnsHandler     *FakeDnsHandler
		)

		BeforeEach(func() {
			fakeHandlerFactory = &FakeHandlerFactory{}

			fakeDnsHandler = &FakeDnsHandler{}
			fakeJsonHandler = &FakeDnsHandler{}

			fakeHandlerFactory.CreateHTTPJSONHandlerReturns(fakeJsonHandler)
			fakeHandlerFactory.CreateForwardHandlerReturns(fakeDnsHandler)
		})

		Context("with no handlers configured", func() {
			It("returns an empty set", func() {
				handlersConfig = HandlersConfig{
					Handlers: []HandlerConfig{},
				}

				handlers, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(handlers)).To(Equal(0))
			})
		})

		Context("with a handler configuration", func() {
			Context("of http type", func() {
				BeforeEach(func() {
					handlersConfig = HandlersConfig{
						Handlers: []HandlerConfig{
							{
								Domain: "my-tld.",
								Source: Source{
									Type: "http",
									URL:  "some-url",
								},
							},
						},
					}
				})

				It("loads the configuration from the url", func() {
					handlers, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(handlers)).To(Equal(1))
					Expect(handlers["my-tld."]).To(Equal(fakeJsonHandler))

					url, enableCache := fakeHandlerFactory.CreateHTTPJSONHandlerArgsForCall(0)
					Expect(url).To(Equal("some-url"))
					Expect(enableCache).To(Equal(false))
				})

				Context("with cache enabled", func() {
					BeforeEach(func() {
						handlersConfig.Handlers[0].Cache.Enabled = true
					})

					It("passes that option to the factory", func() {
						handlers, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(handlers)).To(Equal(1))

						url, enableCache := fakeHandlerFactory.CreateHTTPJSONHandlerArgsForCall(0)
						Expect(url).To(Equal("some-url"))
						Expect(enableCache).To(Equal(true))
					})
				})

				Context("but with no URL setup", func() {
					BeforeEach(func() {
						handlersConfig.Handlers[0].Source.URL = ""
					})

					It("produces an error", func() {
						_, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(`Configuring handler for "my-tld.": HTTP handler must receive a URL`))
					})
				})
			})

			Context("of dns type", func() {
				BeforeEach(func() {
					handlersConfig = HandlersConfig{
						Handlers: []HandlerConfig{
							{
								Domain: "my-tld.",
								Source: Source{
									Type:      "dns",
									Recursors: []string{"some-recursor", "another-recursor"},
								},
							},
						},
					}
				})

				It("loads the configuration from the url", func() {
					handlers, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(handlers)).To(Equal(1))
					Expect(handlers["my-tld."]).To(Equal(fakeDnsHandler))

					recursors, enableCache := fakeHandlerFactory.CreateForwardHandlerArgsForCall(0)
					Expect(recursors).To(Equal([]string{"some-recursor", "another-recursor"}))
					Expect(enableCache).To(Equal(false))
				})

				Context("but with no recursors declared", func() {
					BeforeEach(func() {
						handlersConfig.Handlers[0].Source.Recursors = []string{}
					})

					It("produces an error", func() {
						_, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(`Configuring handler for "my-tld.": No recursors present`))
					})
				})
			})

			Context("with any other type", func() {
				It("produces an error", func() {
					handlersConfig = HandlersConfig{
						Handlers: []HandlerConfig{
							{
								Domain: "my-tld.",
								Source: Source{
									Type: "badthing",
								},
							},
						},
					}

					_, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`Configuring handler for "my-tld.": Unexpected handler source type: badthing`))
				})
			})
		})

		Context("with multiple handlers configured", func() {
			BeforeEach(func() {
				handlersConfig = HandlersConfig{
					Handlers: []HandlerConfig{
						{
							Domain: "my-tld.",
							Source: Source{
								Type:      "dns",
								Recursors: []string{"some-recursor", "another-recursor"},
							},
						}, {
							Domain: "my-other-tld.",
							Source: Source{
								Type: "http",
								URL:  "some-url",
							},
						},
					},
				}
			})

			It("creates a handler for every configuration", func() {
				handlers, err := handlersConfig.GenerateHandlers(fakeHandlerFactory)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(handlers)).To(Equal(2))
				Expect(handlers["my-tld."]).To(Equal(fakeDnsHandler))
				Expect(handlers["my-other-tld."]).To(Equal(fakeJsonHandler))
			})
		})
	})
})

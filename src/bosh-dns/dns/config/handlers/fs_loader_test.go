package handlers_test

import (
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/config"
	. "bosh-dns/dns/config/handlers"
)

var _ = Describe("FSLoader", func() {
	var parser FSLoader
	var fs *boshsysfakes.FakeFileSystem

	BeforeEach(func() {
		fs = boshsysfakes.NewFakeFileSystem()
		parser = NewFSLoader(fs)
	})

	Describe("Load", func() {
		Context("valid file", func() {
			It("parses the file", func() {
				Expect(fs.WriteFileString("/test/handlers.json",
					`[
					{
						"domain": "local.internal.",
						"cache": { "enabled": true },
						"source": { "type": "http", "url": "http://some.endpoint.local"}
					},
					{
						"domain": "local.internal2.",
						"cache": { "enabled": false },
						"source": { "type": "dns", "recursors": ["127.0.0.1:42"]}
					}
				]`)).To(Succeed())

				handlers, err := parser.Load("/test/handlers.json")
				Expect(err).ToNot(HaveOccurred())

				config := HandlerConfigs{
					{
						Domain: "local.internal.",
						Cache:  config.Cache{Enabled: true},
						Source: Source{Type: "http", URL: "http://some.endpoint.local", Recursors: []string{}},
					},
					{
						Domain: "local.internal2.",
						Cache:  config.Cache{Enabled: false},
						Source: Source{Type: "dns", Recursors: []string{"127.0.0.1:42"}},
					},
				}

				Expect(handlers).To(Equal(config))
			})

			It("it rewrites source recursors to include default ports", func() {
				Expect(fs.WriteFileString("/test/handlers.json",
					`[
					{
						"domain": "local.internal2.",
						"cache": { "enabled": false },
						"source": { "type": "dns", "recursors": [ "169.254.169.254", "10.244.4.4:9700" ] }
					}
				]`)).To(Succeed())

				config, err := parser.Load("/test/handlers.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(config[0].Source.Recursors).To(ContainElement("169.254.169.254:53"))
				Expect(config[0].Source.Recursors).To(ContainElement("10.244.4.4:9700"))
			})
		})

		Context("missing file", func() {
			It("errors", func() {
				_, err := parser.Load("/test/handlers.json")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

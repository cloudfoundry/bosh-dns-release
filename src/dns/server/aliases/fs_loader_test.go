package aliases_test

import (
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FSLoader", func() {
	var parser NamedConfigLoader
	var fs *boshsysfakes.FakeFileSystem

	BeforeEach(func() {
		fs = boshsysfakes.NewFakeFileSystem()
		parser = NewFSLoader(fs)
	})

	Describe("Load", func() {
		Context("valid file", func() {
			It("parses the file", func() {
				fs.WriteFileString("/test/aliases.json", `{"test.tld":["othertest.tld"],"test1.tld":["test2.tld","test3.tld"]}`)

				aliases, err := parser.Load("/test/aliases.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(aliases).To(Equal(Config{
					"test.tld": []string{
						"othertest.tld",
					},
					"test1.tld": []string{
						"test2.tld",
						"test3.tld",
					},
				}))
			})
		})

		Context("missing file", func() {
			It("errors", func() {
				_, err := parser.Load("/test/aliases.json")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

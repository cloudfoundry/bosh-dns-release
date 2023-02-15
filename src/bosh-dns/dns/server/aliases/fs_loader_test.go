package aliases_test

import (
	. "bosh-dns/dns/server/aliases"

	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/onsi/ginkgo/v2"
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
				fs.WriteFileString("/test/aliases.json", //nolint:errcheck
					`
				{
					"test.tld": [
						"othertest.tld"
					],
					"test1.tld":[
						"test2.tld",
						"test3.tld"
					]
				}`)

				aliases, err := parser.Load("/test/aliases.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(aliases).To(Equal(MustNewConfigFromMap(map[string][]string{
					"test.tld.": {
						"othertest.tld.",
					},
					"test1.tld.": {
						"test2.tld.",
						"test3.tld.",
					},
				})))
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

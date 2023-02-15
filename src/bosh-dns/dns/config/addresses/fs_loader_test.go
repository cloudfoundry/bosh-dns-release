package addresses_test

import (
	. "bosh-dns/dns/config/addresses"

	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
				fs.WriteFileString("/test/addresses.json", //nolint:errcheck
					`[
					{
						"address": "10.0.14.4",
						"port": 53
					},
					{
						"address": "172.13.3.5",
						"port": 51
					}
				]`)

				addresses, err := parser.Load("/test/addresses.json")
				Expect(err).ToNot(HaveOccurred())

				config := AddressConfigs{
					{
						Address: "10.0.14.4",
						Port:    53,
					},
					{
						Address: "172.13.3.5",
						Port:    51,
					},
				}

				Expect(addresses).To(Equal(config))
			})
		})

		Context("missing port", func() {
			It("errors", func() {
				fs.WriteFileString("/test/addresses.json", //nolint:errcheck
					`[
					{
						"address": "10.0.14.4",
						"port": 53
					},
					{
						"address": "172.13.3.5"
					}
				]`)

				_, err := parser.Load("/test/addresses.json")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("missing file", func() {
			It("errors", func() {
				_, err := parser.Load("/test/addresses.json")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

package records_test

import (
	"encoding/json"

	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecordSet", func() {
	var recordSet records.RecordSet
	Context("when there are records matching the query based fqdn", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
			{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
				"record_infos": [
				["instance0", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "my-domain"],
				["instance1", "my-group", "az2", "my-network", "my-deployment", "123.123.123.124", "my-domain"],
				["instance2", "my-group-2", "az1", "my-network", "my-deployment", "123.123.123.125", "my-domain"],
				["instance4", "my-group", "az1", "another-network", "my-deployment", "123.123.123.127", "my-domain"]
				]
			}
			`)
			err := json.Unmarshal(jsonBytes, &recordSet)

			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the query is for 'all'", func() {
			It("returns all records matching the my-group.my-network.my-deployment.my-domain portion of the fqdn", func() {
				// b64(all) ==  YWxs
				ips, err := recordSet.Resolve("q-YWxs.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("123.123.123.123"))
				Expect(ips).To(ContainElement("123.123.123.124"))
			})
		})

		Context("when the query is for anything but all", func() {
			It("returns an empty set", func() {
				// b64(potato) ==  cG90YXRv
				ips, err := recordSet.Resolve("q-cG90YXRv.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ips)).To(Equal(0))
			})
		})

		Context("when the query fails to decode", func() {
			It("returns an empty set", func() {
				// garbage string ==  )(*&)(*&)(*&)(*&
				ips, err := recordSet.Resolve("q- )(*&)(*&)(*&)(*&.my-group.my-network.my-deployment.my-domain.")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("illegal base64 data at input byte 0"))
				Expect(len(ips)).To(Equal(0))
			})
		})

		Context("when the query is standard base64-encoded", func() {
			It("returns an empty set", func() {
				// b64(Ma~) == TWF+Cg==
				_, err := recordSet.Resolve("q-TWF+Cg==.my-group.my-network.my-deployment.my-domain.")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("illegal base64 data at input byte 3"))
			})
		})

		Context("when the query is base64-raw-URL encoded", func() {
			It("returns an empty set", func() {
				// b64_rawURL(Ma~) == TWF-Cg
				ips, err := recordSet.Resolve("q-TWF-Cg.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ips)).To(Equal(0))
			})
		})
	})

	Context("when there are records matching the specified fqdn", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
{
	"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
	"record_infos": [
		["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "potato"],
		["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124", "potato"]
	]
}
			`)
			err := json.Unmarshal(jsonBytes, &recordSet)

			Expect(err).ToNot(HaveOccurred())
		})

		It("returns all records for that name", func() {
			records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.potato.")
			Expect(err).NotTo(HaveOccurred())

			Expect(records).To(ContainElement("123.123.123.123"))
			Expect(records).To(ContainElement("123.123.123.124"))
		})

		Context("when there are no records matching the specified domain", func() {
			It("returns an empty set of records", func() {
				records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.wrong-domain.")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(HaveLen(0))
			})
		})

		Context("when there are no records matching the specified fqdn", func() {
			It("returns an empty set of records", func() {
				records, err := recordSet.Resolve("some.garbage.fqdn.deploy.potato")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(HaveLen(0))
			})
		})
	})

	Describe("UnmarshalJSON", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
				"record_infos": [
				["instance0", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "withadot."],
				["instance1", "my-group", "az2", "my-network", "my-deployment", "123.123.123.124", "nodot"]
				]
			}`)
			err := json.Unmarshal(jsonBytes, &recordSet)

			Expect(err).ToNot(HaveOccurred())
		})

		It("normalizes domain names", func() {
			Expect(recordSet.Domains).To(ConsistOf("withadot.", "nodot."))
			Expect(recordSet.Records).To(Equal([]records.Record{
				{
					Id:         "instance0",
					Group:      "my-group",
					Network:    "my-network",
					Deployment: "my-deployment",
					Ip:         "123.123.123.123",
					Domain:     "withadot.",
				},
				{
					Id:         "instance1",
					Group:      "my-group",
					Network:    "my-network",
					Deployment: "my-deployment",
					Ip:         "123.123.123.124",
					Domain:     "nodot.",
				},
			}))
		})
	})
})

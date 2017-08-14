package records_test

import (
	"encoding/json"

	"bosh-dns/dns/server/records"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecordSet", func() {
	var recordSet records.RecordSet
	Context("when there are records matching the query based fqdn", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
			{
				"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
				"record_infos": [
				["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "my-domain", 1],
				["instance1", "my-group", "az2", "2", "my-network", "my-deployment", "123.123.123.124", "my-domain", 2],
				["instance1", "my-group", "az3", "3", "my-network", "my-deployment", "123.123.123.126", "my-domain", 0],
				["instance2", "my-group-2", "az1", "1", "my-network", "my-deployment", "123.123.123.125", "my-domain", 1],
				["instance4", "my-group", "az4", "4", "another-network", "my-deployment", "123.123.123.127", "my-domain", 0]
				]
			}
			`)
			err := json.Unmarshal(jsonBytes, &recordSet)

			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the query is for 'status=healthy'", func() {
			It("returns all records matching the my-group.my-network.my-deployment.my-domain portion of the fqdn", func() {
				ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(3))
				Expect(ips).To(ContainElement("123.123.123.123"))
				Expect(ips).To(ContainElement("123.123.123.124"))
				Expect(ips).To(ContainElement("123.123.123.126"))
			})
		})

		Context("when the query contains poorly formed contents", func() {
			It("returns an empty set", func() {
				ips, err := recordSet.Resolve("q-missingvalue.my-group.my-network.my-deployment.my-domain.")
				Expect(err).To(HaveOccurred())
				Expect(len(ips)).To(Equal(0))
			})
		})

		Context("when the query does not include any filters", func() {
			It("returns all records matching the my-group.my-network.my-deployment.my-domain portion of the fqdn", func() {
				ips, err := recordSet.Resolve("q-.my-group.my-network.my-deployment.my-domain.")
				Expect(err).To(HaveOccurred())
				Expect(ips).To(HaveLen(0))
			})
		})

		Context("when the query includes unrecognized filters", func() {
			It("returns an empty set", func() {
				ips, err := recordSet.Resolve("q-x1.my-group.my-network.my-deployment.my-domain.")
				Expect(err).To(HaveOccurred())
				Expect(len(ips)).To(Equal(0))
			})
		})

		Describe("filtering by AZ", func() {
			Context("when the query includes an az id", func() {
				It("only returns records that are in that az", func() {
					ips, err := recordSet.Resolve("q-a1.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(1))
					Expect(ips).To(ContainElement("123.123.123.123"))
				})
			})

			Context("when the query includes multiple az ids", func() {
				It("returns records that are in any of those azs", func() {
					ips, err := recordSet.Resolve("q-a1a3.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(2))
					Expect(ips).To(ContainElement("123.123.123.123"))
					Expect(ips).To(ContainElement("123.123.123.126"))
				})
			})

			Context("when the query includes an AZ index that isn't known", func() {
				It("returns an empty set", func() {
					ips, err := recordSet.Resolve("q-a6.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(len(ips)).To(Equal(0))
				})
			})
		})
	})

	Context("when there are records matching the specified fqdn", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
{
	"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain"],
	"record_infos": [
		["my-instance", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "potato"],
		["my-instance", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.124", "potato"]
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

	Context("when fqdn is already an IP address", func() {
		It("return the IP back", func() {
			records, err := recordSet.Resolve("123.123.123.123")
			Expect(err).NotTo(HaveOccurred())

			Expect(records).To(ContainElement("123.123.123.123"))
		})
	})

	Describe("UnmarshalJSON", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain"],
				"record_infos": [
				["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "withadot."],
				["instance1", "my-group", "az2", "2", "my-network", "my-deployment", "123.123.123.124", "nodot"]
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
					AzId:       "1",
				},
				{
					Id:         "instance1",
					Group:      "my-group",
					Network:    "my-network",
					Deployment: "my-deployment",
					Ip:         "123.123.123.124",
					Domain:     "nodot.",
					AzId:       "2",
				},
			}))
		})
	})
})

package records_test

import (
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecordSet", func() {
	var recordSet records.RecordSet
	Context("when there are records matching the query based fqdn", func() {

		BeforeEach(func() {
			recordSet = records.RecordSet{
				Keys: []string{"id", "instance_group", "az", "network", "deployment", "ip"},
				Infos: [][]string{
					{"instance0", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"},
					{"instance1", "my-group", "az2", "my-network", "my-deployment", "123.123.123.124"},

					{"instance2", "my-group-2", "az1", "my-network", "my-deployment", "123.123.123.125"},
					{"instance4", "my-group", "az1", "another-network", "my-deployment", "123.123.123.127"},
					{"instance5", "my-group", "az1", "my-network", "deployment2", "123.123.123.128"},
				},
			}
		})

		Context("when the query is for 'all'", func() {
			It("returns all records matching the my-group.my-network.my-deployment.bosh portion of the fqdn", func() {
				// b64(all) ==  YWxs
				ips, err := recordSet.Resolve("q-YWxs.my-group.my-network.my-deployment.bosh.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("123.123.123.123"))
				Expect(ips).To(ContainElement("123.123.123.124"))
			})
		})

		Context("when the query is for anything but all", func() {
			It("returns an empty set", func() {
				// b64(potato) ==  cG90YXRv
				ips, err := recordSet.Resolve("q-cG90YXRv.my-group.my-network.my-deployment.bosh.")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ips)).To(Equal(0))
			})
		})

		Context("when the query fails to decode", func() {
			It("returns an empty set", func() {
				// garbage string ==  )(*&)(*&)(*&)(*&
				ips, err := recordSet.Resolve("q- )(*&)(*&)(*&)(*&.my-group.my-network.my-deployment.bosh.")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("illegal base64 data at input byte 0"))
				Expect(len(ips)).To(Equal(0))
			})
		})
	})

	Context("when there are records matching the specified fqdn", func() {
		BeforeEach(func() {
			recordSet = records.RecordSet{
				Keys: []string{"id", "instance_group", "az", "network", "deployment", "ip"},
				Infos: [][]string{
					{"my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"},
					{"my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124"},
				},
			}
		})

		It("returns all records for that name", func() {
			records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.bosh.")
			Expect(err).NotTo(HaveOccurred())

			Expect(records).To(ContainElement("123.123.123.123"))
			Expect(records).To(ContainElement("123.123.123.124"))
		})

		Context("when there are no records matching the specified fqdn", func() {
			It("returns an empty set of records", func() {
				records, err := recordSet.Resolve("some.garbage.fqdn.deploy.bosh")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(HaveLen(0))
			})
		})
	})
})

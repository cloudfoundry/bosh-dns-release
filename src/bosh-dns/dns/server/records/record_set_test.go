package records_test

import (
	"bosh-dns/dns/server/records"
	"reflect"
	"strings"

	"fmt"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecordSet", func() {
	var err error
	var recordSet records.RecordSet
	var fakeLogger *fakes.FakeLogger

	BeforeEach(func() {
		fakeLogger = &fakes.FakeLogger{}
	})

	Context("the records json contains invalid info lines", func() {
		DescribeTable("one of the info lines contains an object",
			func(invalidJson string, logValueIdx int, logValueName string, logExpectedType string) {
				jsonBytes := []byte(fmt.Sprintf(`
		{
			"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
			"record_infos": [
				["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "my-domain", 1],
				%s
			]
		}
				`, invalidJson))

				recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)
				Expect(err).ToNot(HaveOccurred())

				ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(1))
				Expect(ips).To(ContainElement("123.123.123.123"))

				Expect(fakeLogger.WarnCallCount()).To(Equal(1))
				logTag, _, logArgs := fakeLogger.WarnArgsForCall(0)
				Expect(logTag).To(Equal("RecordSet"))
				Expect(logArgs[0]).To(Equal(logValueIdx))
				Expect(logArgs[1]).To(Equal(logValueName))
				Expect(logArgs[2]).To(Equal(1))
				Expect(logArgs[3]).To(Equal(logExpectedType))
			},
			Entry("Domain is not a string", `["instance1", "my-group", "az2", "2", "my-network", "my-deployment", "123.123.123.124", { "foo": "bar" }, 2]`, 7, "domain", "string"),
			Entry("ID is not a string", `[{"id": "id"}, "my-group", "z3", "3", "my-network", "my-deployment", "123.123.123.126", "my-domain", 0]`, 0, "id", "string"),
			Entry("Group is not a string", `["instance1", {"my-group": "my-group"}, "z3", "3", "my-network", "my-deployment", "123.123.123.126", "my-domain", 0]`, 1, "group", "string"),
			Entry("Network is not a string", `["instance1", "my-group", "z3", "3", {"network": "my-network"}, "my-deployment", "123.123.123.126", "my-domain", 0]`, 4, "network", "string"),
			Entry("Deployment is not a string", `["instance1", "my-group", "z3", "3", "my-network", {"deployment": "my-deployment" }, "123.123.123.126", "my-domain", 0]`, 5, "deployment", "string"),
		)

		Context("the columns do not match", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`
			{
				"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
				"record_infos": [
					["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "my-domain", 1],
					["instance1", "my-group", "my-group", "az2", "2", "my-network", "my-deployment", "123.123.123.124", "my-domain", 2],
					["instance1", "my-group", "az3", "3", "my-network", "my-deployment", "123.123.123.126", "my-domain", 0]
				]
			}
			`)
				recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

				Expect(err).ToNot(HaveOccurred())
			})

			It("does not blow up, logs the invalid record, and returns the info that was parsed correctly", func() {
				ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("123.123.123.123"))
				Expect(ips).To(ContainElement("123.123.123.126"))
				Expect(fakeLogger.WarnCallCount()).To(Equal(1))
			})
		})

		DescribeTable("missing required columns", func(column string) {
			recordKeys := map[string]string{
				"id":             "id",
				"instance_group": "instance_group",
				"network":        "network",
				"deployment":     "deployment",
				"ip":             "ip",
				"domain":         "domain",
			}
			delete(recordKeys, column)
			keys := []string{}
			values := []string{}
			for k, v := range recordKeys {
				keys = append(keys, fmt.Sprintf(`"%s"`, k))
				values = append(values, fmt.Sprintf(`"%s"`, v))
			}
			jsonBytes := []byte(fmt.Sprintf(`{
				"record_keys": [%s],
				"record_infos": [[%s]]
			}`, strings.Join(keys, ","), strings.Join(values, ",")))
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).To(BeEmpty())
		},
			Entry("missing id", "id"),
			Entry("missing instance_group", "instance_group"),
			Entry("missing network", "network"),
			Entry("missing deployment", "deployment"),
			Entry("missing ip", "ip"),
			Entry("missing domainn", "domain"),
		)

		DescribeTable("missing optional columns", func(column, field string) {
			recordKeys := map[string]string{
				"id":             "id",
				"instance_group": "instance_group",
				"network":        "network",
				"deployment":     "deployment",
				"ip":             "ip",
				"domain":         "domain",

				"az_id": "az_id",
			}
			delete(recordKeys, column)
			keys := []string{}
			values := []string{}
			for k, v := range recordKeys {
				keys = append(keys, fmt.Sprintf(`"%s"`, k))
				values = append(values, fmt.Sprintf(`"%s"`, v))
			}
			jsonBytes := []byte(fmt.Sprintf(`{
				"record_keys": [%s],
				"record_infos": [[%s]]
			}`, strings.Join(keys, ","), strings.Join(values, ",")))
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).NotTo(BeEmpty())

			value := reflect.ValueOf(&recordSet.Records[0]).Elem().FieldByName(field)
			Expect(value.String()).To(BeEmpty())
		},
			Entry("missing az_id", "az_id", "AZID"),
		)
	})

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
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

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
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

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

	Describe("CreateFromJSON", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain"],
				"record_infos": [
				["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "withadot."],
				["instance1", "my-group", "az2", "2", "my-network", "my-deployment", "123.123.123.124", "nodot"],
				["instance2", "my-group", "az3", null, "my-network", "my-deployment", "123.123.123.125", "domain."],
				["instance3", "my-group", null, "3", "my-network", "my-deployment", "123.123.123.126", "domain."],
				["instance4", "my-group", null, null, "my-network", "my-deployment", "123.123.123.127", "domain."]
				]
			}`)
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("normalizes domain names", func() {
			Expect(recordSet.Domains).To(ConsistOf("withadot.", "nodot.", "domain."))
			Expect(recordSet.Records).To(ContainElement(records.Record{
				ID:         "instance0",
				Group:      "my-group",
				Network:    "my-network",
				Deployment: "my-deployment",
				IP:         "123.123.123.123",
				Domain:     "withadot.",
				AZID:       "1",
			}))
			Expect(recordSet.Records).To(ContainElement(records.Record{
				ID:         "instance1",
				Group:      "my-group",
				Network:    "my-network",
				Deployment: "my-deployment",
				IP:         "123.123.123.124",
				Domain:     "nodot.",
				AZID:       "2",
			}))
		})

		It("does not ignore null azs", func() {
			Expect(recordSet.Records).To(ContainElement(records.Record{
				ID:         "instance2",
				Group:      "my-group",
				Network:    "my-network",
				Deployment: "my-deployment",
				IP:         "123.123.123.125",
				Domain:     "domain.",
				AZID:       "",
			}))
			Expect(recordSet.Records).To(ContainElement(records.Record{
				ID:         "instance3",
				Group:      "my-group",
				Network:    "my-network",
				Deployment: "my-deployment",
				IP:         "123.123.123.126",
				Domain:     "domain.",
				AZID:       "3",
			}))
			Expect(recordSet.Records).To(ContainElement(records.Record{
				ID:         "instance4",
				Group:      "my-group",
				Network:    "my-network",
				Deployment: "my-deployment",
				IP:         "123.123.123.127",
				Domain:     "domain.",
				AZID:       "",
			}))
		})
	})
})

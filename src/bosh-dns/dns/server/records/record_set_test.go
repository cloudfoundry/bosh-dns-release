package records_test

import (
	"bosh-dns/dns/server/records"
	"strings"

	"fmt"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func dereferencer(r []*records.Record) []records.Record {
	out := []records.Record{}
	for _, pointer := range r {
		out = append(out, *pointer)
	}

	return out
}

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
			"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
			"record_infos": [
			["instance0", "2", "my-group", ["3"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
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
			Entry("Domain is not a string", `["instance1", "3", "my-group", ["6"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", { "foo": "bar" }, 2]`, 10, "domain", "string"),
			Entry("ID is not a string", `[{"id": "id"}, "3", "my-group", ["6"], "z3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 0, "id", "string"),
			Entry("Group is not a string", `["instance1", "3", {"my-group": "my-group"}, ["6"], "z3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 2, "group", "string"),
			Entry("Network is not a string", `["instance1", "3", "my-group", ["6"], "z3", "3", {"network": "my-network"}, "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 6, "network", "string"),
			Entry("Deployment is not a string", `["instance1", "3", "my-group", ["6"], "z3", "3", "my-network", "1", {"deployment": "my-deployment" }, "123.123.123.126", "my-domain", 0]`, 8, "deployment", "string"),
			Entry("Group IDs is not an array of string", `["instance1", "3", "my-group", {"6":3}, "z3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 3, "group_ids", "array of string"),
			Entry("Group IDs is not an array of string", `["instance1", "3", "my-group", [3], "z3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 3, "group_ids", "array of string"),

			Entry("Global Index is not a string", `["instance1", {"instance_id": "instance_id"}, "my-group", ["6"], "z3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0]`, 1, "num_id", "string"),
			Entry("Network ID is not a string", `["instance1", "4", "my-group", ["6"], "z3", "3", "my-network", {"network": "invalid"}, "my-deployment", "123.123.123.126", "my-domain", 0]`, 7, "network_id", "string"),
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
			Entry("missing domain", "domain"),
		)

		It("includes records that are well-formed but missing individual group_ids values", func() {
			jsonBytes := []byte(`{
					"record_keys": ["id", "instance_group", "group_ids", "network", "deployment", "ip", "domain"],
					"record_infos": [
						["id", "instance_group", [], "network", "deployment", "ip", "domain"]
					]
				}`)
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(recordSet.Records).ToNot(BeEmpty())
		})

		It("allows for a missing az_id", func() {
			recordKeys := map[string]interface{}{
				"id":             "id",
				"instance_group": "instance_group",
				"group_ids":      []string{"3"},
				"network":        "network",
				"deployment":     "deployment",
				"ip":             "ip",
				"domain":         "domain",
				"instance_index": 1,
			}
			keys := []string{}
			values := []string{}
			for k, v := range recordKeys {
				keys = append(keys, fmt.Sprintf(`"%s"`, k))
				switch typed := v.(type) {
				case int:
					values = append(values, fmt.Sprintf(`%d`, typed))
				case string:
					values = append(values, fmt.Sprintf(`"%s"`, typed))
				case []string:
					values = append(values, fmt.Sprintf(`["%s"]`, typed[0]))
				}
			}
			jsonBytes := []byte(fmt.Sprintf(`{
				"record_keys": [%s],
				"record_infos": [[%s]]
			}`, strings.Join(keys, ","), strings.Join(values, ",")))
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).NotTo(BeEmpty())

			Expect(recordSet.Records[0].AZID).To(Equal(""))
		})

		It("allows for a missing instance_index when the header is missing", func() {
			recordKeys := map[string]interface{}{
				"id":             "id",
				"instance_group": "instance_group",
				"group_ids":      []string{"3"},
				"network":        "network",
				"deployment":     "deployment",
				"ip":             "ip",
				"domain":         "domain",
				"az_id":          "az_id",
			}
			keys := []string{}
			values := []string{}
			for k, v := range recordKeys {
				keys = append(keys, fmt.Sprintf(`"%s"`, k))
				switch typed := v.(type) {
				case int:
					values = append(values, fmt.Sprintf(`%d`, typed))
				case string:
					values = append(values, fmt.Sprintf(`"%s"`, typed))
				case []string:
					values = append(values, fmt.Sprintf(`["%s"]`, typed[0]))
				}
			}
			jsonBytes := []byte(fmt.Sprintf(`{
				"record_keys": [%s],
				"record_infos": [[%s]]
			}`, strings.Join(keys, ","), strings.Join(values, ",")))
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).NotTo(BeEmpty())

			Expect(recordSet.Records[0].InstanceIndex).To(Equal(""))
		})

		It("allows for a missing group_ids when the header is missing", func() {
			recordKeys := map[string]interface{}{
				"id":             "id",
				"instance_group": "instance_group",
				"instance_index": 0,
				"network":        "network",
				"deployment":     "deployment",
				"ip":             "ip",
				"domain":         "domain",
				"az_id":          "az_id",
			}
			keys := []string{}
			values := []string{}
			for k, v := range recordKeys {
				keys = append(keys, fmt.Sprintf(`"%s"`, k))
				switch typed := v.(type) {
				case int:
					values = append(values, fmt.Sprintf(`%d`, typed))
				case string:
					values = append(values, fmt.Sprintf(`"%s"`, typed))
				case []string:
					values = append(values, fmt.Sprintf(`["%s"]`, typed[0]))
				}
			}
			jsonBytes := []byte(fmt.Sprintf(`{
				"record_keys": [%s],
				"record_infos": [[%s]]
			}`, strings.Join(keys, ","), strings.Join(values, ",")))
			recordSet, err := records.CreateFromJSON(jsonBytes, fakeLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(recordSet.Records).NotTo(BeEmpty())

			Expect(recordSet.Records[0].GroupIDs).To(BeEmpty())
		})
	})

	Context("when there are records matching the query based fqdn", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
			{
				"record_keys":
					["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
				"record_infos": [
					["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
					["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "my-domain", 2],
					["instance1", "2", "my-group", ["1"], "az3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0],
					["instance2", "3", "my-group-2", ["2"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.125", "my-domain", 1],
					["instance4", "4", "my-group", ["1"], "az4", "4", "another-network", "2", "my-deployment", "123.123.123.127", "my-domain", 0]
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

			It("interprets short group queries the same way", func() {
				ips, err := recordSet.Resolve("q-s0.q-g1.my-domain.")
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(4))
				Expect(ips).To(ContainElement("123.123.123.123"))
				Expect(ips).To(ContainElement("123.123.123.124"))
				Expect(ips).To(ContainElement("123.123.123.126"))
				Expect(ips).To(ContainElement("123.123.123.127"))
			})
		})

		It("can find specific instances using short group queries", func() {
			ips, err := recordSet.Resolve("instance0.q-g1.my-domain.")
			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveLen(1))
			Expect(ips).To(ContainElement("123.123.123.123"))
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

		Describe("filtering by index", func() {
			Context("when the query includes a single index", func() {
				It("only returns records that have the index", func() {
					ips, err := recordSet.Resolve("q-i2.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(1))
					Expect(ips).To(ContainElement("123.123.123.124"))
				})
			})

			Context("when the query includes a single index", func() {
				It("only returns records that have the index", func() {
					ips, err := recordSet.Resolve("q-i2i0.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(2))
					Expect(ips).To(ContainElement("123.123.123.124"))
					Expect(ips).To(ContainElement("123.123.123.126"))
				})
			})

			Context("when the query includes an index that isn't known", func() {
				It("returns an empty set", func() {
					ips, err := recordSet.Resolve("q-i5.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(0))
				})
			})
		})

		Describe("filtering by global index", func() {
			Context("when the query includes a global index", func() {
				It("only returns records that are in that global index", func() {
					ips, err := recordSet.Resolve("q-m1.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(1))
					Expect(ips).To(ContainElement("123.123.123.124"))
				})
			})

			Context("when the query includes a global index that isn't known", func() {
				It("returns an empty set", func() {
					ips, err := recordSet.Resolve("q-m12.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(len(ips)).To(Equal(0))
				})
			})
		})

		Describe("filtering by network id", func() {
			Context("when the query includes a network ID", func() {
				It("only returns records that are in that network ID", func() {
					ips, err := recordSet.Resolve("q-n1.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(ips).To(HaveLen(3))
					Expect(ips).To(ContainElement("123.123.123.123"))
					Expect(ips).To(ContainElement("123.123.123.124"))
					Expect(ips).To(ContainElement("123.123.123.126"))
				})
			})

			Context("when the query includes a network ID that isn't known", func() {
				It("returns an empty set", func() {
					ips, err := recordSet.Resolve("q-n12.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					Expect(len(ips)).To(Equal(0))
				})
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

		Describe("filtering by both AZ and index", func() {
			/*
				context:
					az1: 0, 1
					az2: 2, 3, 4
					az3: 5
			*/
			BeforeEach(func() {
				jsonBytes := []byte(`
			{
				"record_keys":
					["id", 				"instance_group", "az", "az_id", "network", 		 "deployment", 		"ip", 						 "domain",  	"instance_index"],
				"record_infos": [
					["instance0", "my-group",       "az1", "1",    "my-network", 	 "my-deployment", "123.123.123.123", "my-domain", 0],
					["instance1", "my-group",       "az1", "1",    "my-network", 	 "my-deployment", "123.123.123.124", "my-domain", 1],
					["instance2", "my-group",       "az2", "2",    "my-network", 	 "my-deployment", "123.123.123.125", "my-domain", 2],
					["instance3", "my-group",       "az2", "2",    "my-network", 	 "my-deployment", "123.123.123.126", "my-domain", 3],
					["instance4", "my-group",       "az2", "2",    "my-network", 	 "my-deployment", "123.123.123.127", "my-domain", 4],
					["instance5", "my-group",       "az3", "3",    "my-network", 	 "my-deployment", "123.123.123.128", "my-domain", 5]
				]
			}
			`)
				recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

				Expect(err).ToNot(HaveOccurred())
			})

			It("matches one index across multiple AZs", func() {
				/*
					query: (az2 OR az3) AND i2
					expected: az2 i2
				*/
				ips, err := recordSet.Resolve("q-a2a3i2.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(HaveLen(1))
				Expect(ips).To(ContainElement("123.123.123.125"))
			})

			It("match multiple indexes on one AZ", func() {
				/*
					query: az2 AND (i2 OR i3)
					expected: az2 i2 or i3
				*/
				ips, err := recordSet.Resolve("q-a2i2i3.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("123.123.123.125"))
				Expect(ips).To(ContainElement("123.123.123.126"))
			})

			It("match multiple indexes on multiple AZs", func() {
				/*
					query: (az1 OR az2) AND (i2 OR i1)
					expected: az1 i1, az2 i2
				*/
				ips, err := recordSet.Resolve("q-a1a2i2i1.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(HaveLen(2))
				Expect(ips).To(ContainElement("123.123.123.124"))
				Expect(ips).To(ContainElement("123.123.123.125"))
			})

			It("don't match an non-existent index on multiple AZs", func() {
				/*
					query: (az2 OR az3) AND i0
					expected: nothing
				*/
				ips, err := recordSet.Resolve("q-a2a3i0.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(HaveLen(0))
			})

		})

	})

	Context("when there are records that only differ in domains", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`
			{
				"record_keys":
					["id",        "instance_group", "az", "az_id", "network",    "deployment", 		"ip", 						 "domain",  	"instance_index"],
				"record_infos": [
					["instance0", "my-group",       "az1", "1",    "my-network", "my-deployment", "123.123.123.123", "my-domain", 0],
					["instance0", "my-group",       "az1", "1",    "my-network", "my-deployment", "123.123.123.124", "another-domain", 1]
				]
			}
			`)
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to filter on domain", func() {
			ips, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.another-domain.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(HaveLen(1))
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
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "withadot."],
					["instance1", "1", "my-group", "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "nodot"],
					["instance2", "2", "my-group", "az3", null, "my-network", "1", "my-deployment", "123.123.123.125", "domain."],
					["instance3", "3", "my-group", null, "3", "my-network", "1", "my-deployment", "123.123.123.126", "domain."],
					["instance4", "4", "my-group", null, null, "my-network", "1", "my-deployment", "123.123.123.127", "domain."],
					["instance5", "5", "my-group", null, null, "my-network", null, "my-deployment", "123.123.123.128", "domain."],
					["instance6", null, "my-group", null, null, "my-network", "1", "my-deployment", "123.123.123.129", "domain."]
				]
			}`)
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("normalizes domain names", func() {
			Expect(recordSet.Domains).To(ConsistOf("withadot.", "nodot.", "domain."))
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:          "instance0",
				NumId: "0",
				Group:       "my-group",
				Network:     "my-network",
				NetworkID:   "1",
				Deployment:  "my-deployment",
				IP:          "123.123.123.123",
				Domain:      "withadot.",
				AZID:        "1",
			})))
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:          "instance1",
				NumId: "1",
				Group:       "my-group",
				Network:     "my-network",
				NetworkID:   "1",
				Deployment:  "my-deployment",
				IP:          "123.123.123.124",
				Domain:      "nodot.",
				AZID:        "2",
			})))
		})

		It("includes records with null azs", func() {
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:          "instance2",
				NumId: "2",
				Group:       "my-group",
				Network:     "my-network",
				NetworkID:   "1",
				Deployment:  "my-deployment",
				IP:          "123.123.123.125",
				Domain:      "domain.",
				AZID:        "",
			})))
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:          "instance4",
				NumId: "4",
				Group:       "my-group",
				Network:     "my-network",
				NetworkID:   "1",
				Deployment:  "my-deployment",
				IP:          "123.123.123.127",
				Domain:      "domain.",
				AZID:        "",
			})))
		})

		It("includes records with null instance indexes", func() {
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:            "instance3",
				NumId:   "3",
				Group:         "my-group",
				Network:       "my-network",
				NetworkID:     "1",
				Deployment:    "my-deployment",
				IP:            "123.123.123.126",
				Domain:        "domain.",
				AZID:          "3",
				InstanceIndex: "",
			})))
		})

		It("includes records with no value for network_id", func() {
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:            "instance5",
				NumId:   "5",
				Group:         "my-group",
				Network:       "my-network",
				NetworkID:     "",
				Deployment:    "my-deployment",
				IP:            "123.123.123.128",
				Domain:        "domain.",
				AZID:          "",
				InstanceIndex: "",
			})))
		})

		It("includes records with no value for num_id", func() {
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:            "instance6",
				NumId:   "",
				Group:         "my-group",
				Network:       "my-network",
				NetworkID:     "1",
				Deployment:    "my-deployment",
				IP:            "123.123.123.129",
				Domain:        "domain.",
				AZID:          "",
				InstanceIndex: "",
			})))
		})
	})

	Context("when the records json includes instance_index", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
				"record_infos": [
					["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "domain.", 0],
					["instance1", "my-group", "az2", "1", "my-network", "my-deployment", "123.123.123.124", "domain.", 1]
				]
			}`)
			recordSet, err = records.CreateFromJSON(jsonBytes, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("parses the instance index", func() {
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:            "instance0",
				Group:         "my-group",
				Network:       "my-network",
				Deployment:    "my-deployment",
				IP:            "123.123.123.123",
				Domain:        "domain.",
				AZID:          "1",
				InstanceIndex: "0",
			})))
			Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(records.Record{
				ID:            "instance1",
				Group:         "my-group",
				Network:       "my-network",
				Deployment:    "my-deployment",
				IP:            "123.123.123.124",
				Domain:        "domain.",
				AZID:          "1",
				InstanceIndex: "1",
			})))
		})
	})
})

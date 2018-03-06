package records_test

import (
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"fmt"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func dereferencer(r []record.Record) []record.Record {
	out := []record.Record{}
	for _, record := range r {
		out = append(out, record)
	}

	return out
}

func mustNewConfigFromMap(load map[string][]string) aliases.Config {
	config, err := aliases.NewConfigFromMap(load)
	if err != nil {
		Fail(err.Error())
	}
	return config
}

var _ = Describe("RecordSet", func() {
	var (
		recordSet         *records.RecordSet
		fakeLogger        *fakes.FakeLogger
		fileReader        *recordsfakes.FakeFileReader
		aliasList         aliases.Config
		shutdownChan      chan struct{}
		fakeHealthWatcher *healthinessfakes.FakeHealthWatcher
	)

	BeforeEach(func() {
		fakeLogger = &fakes.FakeLogger{}
		fileReader = &recordsfakes.FakeFileReader{}
		aliasList = mustNewConfigFromMap(map[string][]string{})
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		shutdownChan = make(chan struct{})
	})

	AfterEach(func() {
		close(shutdownChan)
	})

	Describe("Record Set Performance", func() {
		BeforeEach(func() {
			recordData := [][]string{[]string{
				"instance0",
				"0",
				"my-group",
				"az4",
				"4",
				"my-network",
				"1",
				"my-deployment",
				"123.123.1.0",
				"domain.",
			}}

			for i := 1; i < 2000; i++ {
				recordData = append(recordData, []string{
					fmt.Sprintf("instance%d", i),
					fmt.Sprintf("%d", i),
					"my-group",
					fmt.Sprintf("az%d", i%3),
					fmt.Sprintf("%d", i%3),
					"my-network",
					"1",
					"my-deployment",
					fmt.Sprintf("123.123.%d.%d", (i+1)%256, i%256),
					"domain.",
				})
			}

			recordInfosJson, err := json.Marshal(recordData)
			Expect(err).NotTo(HaveOccurred())

			jsonBytes := []byte(fmt.Sprintf(`{
			"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": %s
			}`, recordInfosJson))

			fileReader.GetReturns(jsonBytes, nil)

			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("is able to resolve query with large number of records quickly", func() {
			var totalTime time.Duration
			var totalTimeLastRecord time.Duration
			var count int

			for count = 0; count < 4000; count++ {
				startTime := time.Now()
				ips, err := recordSet.Resolve("q-m0s0.my-group.my-network.my-deployment.domain.")
				totalTime += time.Since(startTime) / time.Microsecond

				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(1))
				Expect(ips).To(ContainElement("123.123.1.0"))

				startTime = time.Now()
				ips, err = recordSet.Resolve("q-m1999s0.my-group.my-network.my-deployment.domain.")
				totalTimeLastRecord += time.Since(startTime) / time.Microsecond

				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(1))
				Expect(ips).To(ContainElement("123.123.208.207"))
			}

			averageTime := totalTime / time.Duration(count)
			averageTimeLastRecord := totalTimeLastRecord / time.Duration(count)

			Expect(averageTime).To(BeNumerically("<", 2000))
			Expect(averageTimeLastRecord).To(BeNumerically("<", 2000))
		})
	})

	Describe("NewRecordSet", func() {
		Context("when the records json includes instance_index", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
									"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
									"record_infos": [
										["instance0", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "domain.", 0],
										["instance1", "my-group", "az2", "1", "my-network", "my-deployment", "123.123.123.124", "domain.", 1]
									]
								}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("parses the instance index", func() {
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance0",
					Group:         "my-group",
					Network:       "my-network",
					Deployment:    "my-deployment",
					IP:            "123.123.123.123",
					Domain:        "domain.",
					AZ:            "az1",
					AZID:          "1",
					InstanceIndex: "0",
				})))
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance1",
					Group:         "my-group",
					Network:       "my-network",
					Deployment:    "my-deployment",
					IP:            "123.123.123.124",
					Domain:        "domain.",
					AZ:            "az2",
					AZID:          "1",
					InstanceIndex: "1",
				})))
			})
		})

		Context("when the records json does not include instance_index", func() {
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
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

				Expect(err).ToNot(HaveOccurred())
			})

			It("normalizes domain names", func() {
				Expect(recordSet.Domains()).To(ConsistOf("withadot.", "nodot.", "domain."))
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:         "instance0",
					NumID:      "0",
					Group:      "my-group",
					Network:    "my-network",
					NetworkID:  "1",
					Deployment: "my-deployment",
					IP:         "123.123.123.123",
					Domain:     "withadot.",
					AZ:         "az1",
					AZID:       "1",
				})))
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:         "instance1",
					NumID:      "1",
					Group:      "my-group",
					Network:    "my-network",
					NetworkID:  "1",
					Deployment: "my-deployment",
					IP:         "123.123.123.124",
					Domain:     "nodot.",
					AZ:         "az2",
					AZID:       "2",
				})))
			})

			It("includes records with null azs", func() {
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:         "instance2",
					NumID:      "2",
					Group:      "my-group",
					Network:    "my-network",
					NetworkID:  "1",
					Deployment: "my-deployment",
					IP:         "123.123.123.125",
					Domain:     "domain.",
					AZ:         "az3",
					AZID:       "",
				})))
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:         "instance4",
					NumID:      "4",
					Group:      "my-group",
					Network:    "my-network",
					NetworkID:  "1",
					Deployment: "my-deployment",
					IP:         "123.123.123.127",
					Domain:     "domain.",
					AZ:         "",
					AZID:       "",
				})))
			})

			It("includes records with null instance indexes", func() {
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance3",
					NumID:         "3",
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
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance5",
					NumID:         "5",
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
				Expect(recordSet.Records).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance6",
					NumID:         "",
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
	})

	Describe("Domains", func() {
		BeforeEach(func() {
			aliasList = mustNewConfigFromMap(map[string][]string{
				"alias1": {""},
			})
		})

		It("returns the domains", func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "withadot."],
					["instance1", "1", "my-group", "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "nodot"],
					["instance2", "2", "my-group", "az3", null, "my-network", "1", "my-deployment", "123.123.123.125", "domain."]
				]
			}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
			Expect(err).ToNot(HaveOccurred())

			Expect(recordSet.Domains()).To(ConsistOf("withadot.", "nodot.", "domain.", "alias1."))
		})
	})

	Describe("HasIP", func() {
		BeforeEach(func() {
			aliasList = mustNewConfigFromMap(map[string][]string{
				"alias1": {""},
			})
		})

		It("returns true if an IP is known", func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "withadot."],
					["instance1", "1", "my-group", "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "nodot"],
					["instance2", "2", "my-group", "az3", null, "my-network", "1", "my-deployment", "123.123.123.125", "domain."]
				]
			}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
			Expect(err).ToNot(HaveOccurred())

			Expect(recordSet.HasIP("123.123.123.123")).To(Equal(true))
			Expect(recordSet.HasIP("127.0.0.1")).To(Equal(false))
		})
	})

	Describe("auto refreshing records", func() {
		var (
			subscriptionChan chan bool
		)

		BeforeEach(func() {
			subscriptionChan = make(chan bool, 1)
			fileReader.SubscribeReturns(subscriptionChan)

			jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "bosh."]
				]
			}`)
			fileReader.GetReturns(jsonBytes, nil)
			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
			Expect(err).ToNot(HaveOccurred())

			ips, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]string{"123.123.123.123"}))
		})

		Context("when updating to valid json", func() {
			var (
				subscribers []<-chan bool
			)

			BeforeEach(func() {
				jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "234.234.234.234", "bosh."]
				]
			}`)
				fileReader.GetReturns(jsonBytes, nil)
				subscriptionChan <- true
				subscribers = append(subscribers, recordSet.Subscribe())
				subscribers = append(subscribers, recordSet.Subscribe())
			})

			It("updates its set of records", func() {
				Eventually(func() []string {
					ips, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())
					return ips
				}).Should(Equal([]string{"234.234.234.234"}))
			})

			It("notifies its own subscribers", func() {
				for _, subscriber := range subscribers {
					Eventually(subscriber).Should(Receive(BeTrue()))
				}
			})
		})

		Context("when the subscription is closed", func() {
			var (
				subscribers []<-chan bool
			)

			BeforeEach(func() {
				subscribers = append(subscribers, recordSet.Subscribe())
				subscribers = append(subscribers, recordSet.Subscribe())
				close(subscriptionChan)
			})

			It("closes all subscribers", func() {
				for _, subscriber := range subscribers {
					Eventually(subscriber).Should(BeClosed())
				}
			})
		})

		Context("when updating to invalid json", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`<invalid>json</invalid>`)
				fileReader.GetReturns(jsonBytes, nil)
				subscriptionChan <- true
			})

			It("keeps the original set of records", func() {
				Consistently(func() []string {
					ips, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())
					return ips
				}).Should(Equal([]string{"123.123.123.123"}))
			})
		})

		Context("when failing to read the file", func() {
			BeforeEach(func() {
				fileReader.GetReturns(nil, errors.New("no read"))
				subscriptionChan <- true
			})

			It("keeps the original set of records", func() {
				Consistently(func() []string {
					ips, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())
					return ips
				}).Should(Equal([]string{"123.123.123.123"}))
			})
		})
	})

	Context("when FileReader returns JSON", func() {
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

					fileReader.GetReturns(jsonBytes, nil)

					var err error
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
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

					fileReader.GetReturns(jsonBytes, nil)

					var err error
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

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

				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

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
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
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
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
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
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
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
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(recordSet.Records).NotTo(BeEmpty())

				Expect(recordSet.Records[0].GroupIDs).To(BeEmpty())
			})
		})
	})

	Describe("Resolve", func() {
		Context("when fqdn is already an IP address", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
									"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
									"record_infos": [
										["instance1", "my-group", "az2", "1", "my-network", "my-deployment", "123.123.123.124", "domain.", 1]
									]
								}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("return the IP back", func() {
				records, err := recordSet.Resolve("123.123.123.123")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(ContainElement("123.123.123.123"))
			})
		})
	})

	Describe("Filter", func() {
		Context("when there are records matching the query based fqdn", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "my-domain", 2],
						["instance1", "2", "my-group", ["1"], "az3", "3", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 0],
						["instance2", "3", "my-group-2", ["2"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.125", "my-domain", 1],
						["instance4", "4", "my-group", ["1"], "az4", "4", "another-network", "2", "my-deployment", "123.123.123.127", "my-domain", 0]
					]
				}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the query is for 'status=healthy'", func() {
				It("returns all records matching the my-group.my-network.my-deployment.my-domain portion of the fqdn", func() {
					rs, err := recordSet.Filter([]string{"q-s0.my-group.my-network.my-deployment.my-domain."}, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(rs).To(HaveLen(3))
					Expect(rs).To(ContainElement(record.Record{
						ID:            "instance0",
						NumID:         "0",
						Group:         "my-group",
						GroupIDs:      []string{"1"},
						Network:       "my-network",
						NetworkID:     "1",
						Deployment:    "my-deployment",
						IP:            "123.123.123.123",
						Domain:        "my-domain.",
						AZ:            "az1",
						AZID:          "1",
						InstanceIndex: "1",
					}))
					Expect(rs).To(ContainElement(record.Record{
						ID:            "instance1",
						NumID:         "1",
						Group:         "my-group",
						GroupIDs:      []string{"1"},
						Network:       "my-network",
						NetworkID:     "1",
						Deployment:    "my-deployment",
						IP:            "123.123.123.124",
						Domain:        "my-domain.",
						AZ:            "az2",
						AZID:          "2",
						InstanceIndex: "2",
					}))
					Expect(rs).To(ContainElement(record.Record{
						ID:            "instance1",
						NumID:         "2",
						Group:         "my-group",
						GroupIDs:      []string{"1"},
						Network:       "my-network",
						NetworkID:     "1",
						Deployment:    "my-deployment",
						IP:            "123.123.123.126",
						Domain:        "my-domain.",
						AZ:            "az3",
						AZID:          "3",
						InstanceIndex: "0",
					}))
				})

				It("interprets short group queries the same way", func() {
					rs, err := recordSet.Filter([]string{"q-s0.q-g1.my-domain."}, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(rs).To(HaveLen(4))
					Expect(rs).To(ContainElement(
						record.Record{
							ID:            "instance0",
							NumID:         "0",
							Group:         "my-group",
							GroupIDs:      []string{"1"},
							Network:       "my-network",
							NetworkID:     "1",
							Deployment:    "my-deployment",
							IP:            "123.123.123.123",
							Domain:        "my-domain.",
							AZ:            "az1",
							AZID:          "1",
							InstanceIndex: "1",
						}))
					Expect(rs).To(ContainElement(
						record.Record{
							ID:            "instance1",
							NumID:         "2",
							Group:         "my-group",
							GroupIDs:      []string{"1"},
							Network:       "my-network",
							NetworkID:     "1",
							Deployment:    "my-deployment",
							IP:            "123.123.123.126",
							Domain:        "my-domain.",
							AZ:            "az3",
							AZID:          "3",
							InstanceIndex: "0",
						}))
					Expect(rs).To(ContainElement(
						record.Record{
							ID:            "instance4",
							NumID:         "4",
							Group:         "my-group",
							GroupIDs:      []string{"1"},
							Network:       "another-network",
							NetworkID:     "2",
							Deployment:    "my-deployment",
							IP:            "123.123.123.127",
							Domain:        "my-domain.",
							AZ:            "az4",
							AZID:          "4",
							InstanceIndex: "0",
						}))
					Expect(rs).To(ContainElement(
						record.Record{
							ID:            "instance1",
							NumID:         "1",
							Group:         "my-group",
							GroupIDs:      []string{"1"},
							Network:       "my-network",
							NetworkID:     "1",
							Deployment:    "my-deployment",
							IP:            "123.123.123.124",
							Domain:        "my-domain.",
							AZ:            "az2",
							AZID:          "2",
							InstanceIndex: "2",
						}))
				})
			})

			It("can find specific instances using short group queries", func() {
				rs, err := recordSet.Filter([]string{"instance0.q-g1.my-domain."}, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(rs).To(HaveLen(1))
				Expect(rs).To(ContainElement(
					record.Record{
						ID:            "instance0",
						NumID:         "0",
						Group:         "my-group",
						GroupIDs:      []string{"1"},
						Network:       "my-network",
						NetworkID:     "1",
						Deployment:    "my-deployment",
						IP:            "123.123.123.123",
						Domain:        "my-domain.",
						AZ:            "az1",
						AZID:          "1",
						InstanceIndex: "1",
					}))
			})

			Context("when the query contains poorly formed contents", func() {
				It("returns an empty set", func() {
					rs, err := recordSet.Filter([]string{"q-missingvalue.my-group.my-network.my-deployment.my-domain."}, true)
					Expect(err).To(HaveOccurred())
					Expect(len(rs)).To(Equal(0))
				})
			})

			Context("when the query does not include any filters", func() {
				It("returns all records matching the my-group.my-network.my-deployment.my-domain portion of the fqdn", func() {
					rs, err := recordSet.Filter([]string{"q-.my-group.my-network.my-deployment.my-domain."}, true)
					Expect(err).To(HaveOccurred())
					Expect(rs).To(HaveLen(0))
				})
			})

			Context("when the query includes unrecognized filters", func() {
				It("returns an empty set", func() {
					rs, err := recordSet.Filter([]string{"q-x1.my-group.my-network.my-deployment.my-domain."}, false)
					Expect(err).To(HaveOccurred())
					Expect(len(rs)).To(Equal(0))
				})
			})

			Describe("filtering by index", func() {
				Context("when the query includes a single index", func() {
					It("only returns records that have the index", func() {
						rs, err := recordSet.Filter([]string{"q-i2.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf(
							[]record.Record{
								{
									ID:            "instance1",
									NumID:         "1",
									Group:         "my-group",
									GroupIDs:      []string{"1"},
									Network:       "my-network",
									NetworkID:     "1",
									Deployment:    "my-deployment",
									IP:            "123.123.123.124",
									Domain:        "my-domain.",
									AZ:            "az2",
									AZID:          "2",
									InstanceIndex: "2",
								},
							},
						))
					})
				})

				Context("when the query includes multiple indices", func() {
					It("only returns records that have those indices", func() {
						rs, err := recordSet.Filter([]string{"q-i2i0.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf([]record.Record{
							{
								ID:            "instance1",
								NumID:         "1",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.124",
								Domain:        "my-domain.",
								AZ:            "az2",
								AZID:          "2",
								InstanceIndex: "2",
							},
							{
								ID:            "instance1",
								NumID:         "2",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.126",
								Domain:        "my-domain.",
								AZ:            "az3",
								AZID:          "3",
								InstanceIndex: "0",
							},
						}))
					})
				})

				Context("when the query includes an index that isn't known", func() {
					It("returns an empty set", func() {
						rs, err := recordSet.Filter([]string{"q-i5.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(HaveLen(0))
					})
				})
			})

			Describe("filtering by global index", func() {
				Context("when the query includes a global index", func() {
					It("only returns records that are in that global index", func() {
						rs, err := recordSet.Filter([]string{"q-m1.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf([]record.Record{
							{
								ID:            "instance1",
								NumID:         "1",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.124",
								Domain:        "my-domain.",
								AZ:            "az2",
								AZID:          "2",
								InstanceIndex: "2",
							},
						}))
					})
				})

				Context("when the query includes a global index that isn't known", func() {
					It("returns an empty set", func() {
						rs, err := recordSet.Filter([]string{"q-m12.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(len(rs)).To(Equal(0))
					})
				})
			})

			Describe("filtering by network id", func() {
				Context("when the query includes a network ID", func() {
					It("only returns records that are in that network ID", func() {
						rs, err := recordSet.Filter([]string{"q-n1.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf([]record.Record{
							{
								ID:            "instance0",
								NumID:         "0",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.123",
								Domain:        "my-domain.",
								AZ:            "az1",
								AZID:          "1",
								InstanceIndex: "1",
							},
							{
								ID:            "instance1",
								NumID:         "1",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.124",
								Domain:        "my-domain.",
								AZ:            "az2",
								AZID:          "2",
								InstanceIndex: "2",
							},
							{
								ID:            "instance1",
								NumID:         "2",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.126",
								Domain:        "my-domain.",
								AZ:            "az3",
								AZID:          "3",
								InstanceIndex: "0",
							},
						}))
					})
				})

				Context("when the query includes a network ID that isn't known", func() {
					It("returns an empty set", func() {
						rs, err := recordSet.Filter([]string{"q-n12.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(len(rs)).To(Equal(0))
					})
				})
			})

			Describe("filtering by AZ", func() {
				Context("when the query includes an az id", func() {
					It("only returns records that are in that az", func() {
						rs, err := recordSet.Filter([]string{"q-a1.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf([]record.Record{
							{
								ID:            "instance0",
								NumID:         "0",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.123",
								Domain:        "my-domain.",
								AZ:            "az1",
								AZID:          "1",
								InstanceIndex: "1",
							},
						}))
					})
				})

				Context("when the query includes multiple az ids", func() {
					It("returns records that are in any of those azs", func() {
						rs, err := recordSet.Filter([]string{"q-a1a3.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(rs).To(ConsistOf([]record.Record{
							{
								ID:            "instance0",
								NumID:         "0",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.123",
								Domain:        "my-domain.",
								AZ:            "az1",
								AZID:          "1",
								InstanceIndex: "1",
							},
							{
								ID:            "instance1",
								NumID:         "2",
								Group:         "my-group",
								GroupIDs:      []string{"1"},
								Network:       "my-network",
								NetworkID:     "1",
								Deployment:    "my-deployment",
								IP:            "123.123.123.126",
								Domain:        "my-domain.",
								AZ:            "az3",
								AZID:          "3",
								InstanceIndex: "0",
							},
						}))
					})
				})

				Context("when the query includes an AZ index that isn't known", func() {
					It("returns an empty set", func() {
						rs, err := recordSet.Filter([]string{"q-a6.my-group.my-network.my-deployment.my-domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(len(rs)).To(Equal(0))
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
					fileReader.GetReturns(jsonBytes, nil)

					var err error
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
					Expect(err).ToNot(HaveOccurred())
				})

				It("matches one index across multiple AZs", func() {
					/*
						query: (az2 OR az3) AND i2
						expected: az2 i2
					*/
					rs, err := recordSet.Filter([]string{"q-a2a3i2.my-group.my-network.my-deployment.my-domain."}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(rs).To(HaveLen(1))
					Expect(rs).To(ConsistOf([]record.Record{
						{
							ID:            "instance2",
							NumID:         "",
							Group:         "my-group",
							GroupIDs:      nil,
							Network:       "my-network",
							NetworkID:     "",
							Deployment:    "my-deployment",
							IP:            "123.123.123.125",
							Domain:        "my-domain.",
							AZ:            "az2",
							AZID:          "2",
							InstanceIndex: "2",
						},
					}))
				})

				It("match multiple indexes on one AZ", func() {
					/*
						query: az2 AND (i2 OR i3)
						expected: az2 i2 or i3
					*/
					rs, err := recordSet.Filter([]string{"q-a2i2i3.my-group.my-network.my-deployment.my-domain."}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(rs).To(ConsistOf([]record.Record{
						{
							ID:            "instance2",
							NumID:         "",
							Group:         "my-group",
							GroupIDs:      nil,
							Network:       "my-network",
							NetworkID:     "",
							Deployment:    "my-deployment",
							IP:            "123.123.123.125",
							Domain:        "my-domain.",
							AZ:            "az2",
							AZID:          "2",
							InstanceIndex: "2",
						},
						{
							ID:            "instance3",
							NumID:         "",
							Group:         "my-group",
							GroupIDs:      nil,
							Network:       "my-network",
							NetworkID:     "",
							Deployment:    "my-deployment",
							IP:            "123.123.123.126",
							Domain:        "my-domain.",
							AZ:            "az2",
							AZID:          "2",
							InstanceIndex: "3",
						},
					}))
				})

				It("match multiple indexes on multiple AZs", func() {
					/*
						query: (az1 OR az2) AND (i2 OR i1)
						expected: az1 i1, az2 i2
					*/
					rs, err := recordSet.Filter([]string{"q-a1a2i2i1.my-group.my-network.my-deployment.my-domain."}, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(rs).To(ConsistOf([]record.Record{
						{
							ID:            "instance1",
							NumID:         "",
							Group:         "my-group",
							GroupIDs:      nil,
							Network:       "my-network",
							NetworkID:     "",
							Deployment:    "my-deployment",
							IP:            "123.123.123.124",
							Domain:        "my-domain.",
							AZ:            "az1",
							AZID:          "1",
							InstanceIndex: "1",
						},
						{
							ID:            "instance2",
							NumID:         "",
							Group:         "my-group",
							GroupIDs:      nil,
							Network:       "my-network",
							NetworkID:     "",
							Deployment:    "my-deployment",
							IP:            "123.123.123.125",
							Domain:        "my-domain.",
							AZ:            "az2",
							AZID:          "2",
							InstanceIndex: "2",
						},
					}))
				})

				It("doesn't match a non-existent index on multiple AZs", func() {
					/*
						query: (az2 OR az3) AND i0
						expected: nothing
					*/
					rs, err := recordSet.Filter([]string{"q-a2a3i0.my-group.my-network.my-deployment.my-domain."}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(rs).To(HaveLen(0))
				})

				Context("when there are records that only differ in domains", func() {
					BeforeEach(func() {
						jsonBytes := []byte(` {
									"record_keys":
										["id",        "instance_group", "az", "az_id", "network",    "deployment", 		"ip", 						 "domain",  	"instance_index"],
									"record_infos": [
										["instance0", "my-group",       "az1", "1",    "my-network", "my-deployment", "123.123.123.123", "my-domain", 0],
										["instance0", "my-group",       "az1", "1",    "my-network", "my-deployment", "123.123.123.124", "another-domain", 1]
									]
								}`)
						fileReader.GetReturns(jsonBytes, nil)

						var err error
						recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

						Expect(err).ToNot(HaveOccurred())
					})

					It("should be able to filter on domain", func() {
						rs, err := recordSet.Filter([]string{"instance0.my-group.my-network.my-deployment.another-domain."}, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(rs).To(HaveLen(1))
					})
				})

				Context("when there are records matching the specified fqdn", func() {
					BeforeEach(func() {
						jsonBytes := []byte(`{
									"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain"],
									"record_infos": [
										["my-instance", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.123", "potato"],
										["my-instance", "my-group", "az1", "1", "my-network", "my-deployment", "123.123.123.124", "potato"]
									]
								}`)
						fileReader.GetReturns(jsonBytes, nil)

						var err error
						recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns an empty set of records", func() {
						records, err := recordSet.Filter([]string{"some.garbage.fqdn.deploy.potato"}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(records).To(HaveLen(0))
					})

					It("returns all records for that name", func() {
						set, err := recordSet.Filter([]string{"my-instance.my-group.my-network.my-deployment.potato."}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(set).To(ConsistOf([]record.Record{
							{
								ID:            "my-instance",
								NumID:         "",
								Group:         "my-group",
								GroupIDs:      nil,
								Network:       "my-network",
								NetworkID:     "",
								Deployment:    "my-deployment",
								IP:            "123.123.123.123",
								Domain:        "potato.",
								AZ:            "az1",
								AZID:          "1",
								InstanceIndex: "",
							},
							{
								ID:            "my-instance",
								NumID:         "",
								Group:         "my-group",
								GroupIDs:      nil,
								Network:       "my-network",
								NetworkID:     "",
								Deployment:    "my-deployment",
								IP:            "123.123.123.124",
								Domain:        "potato.",
								AZ:            "az1",
								AZID:          "1",
								InstanceIndex: "",
							},
						}))
					})

					Context("when there are no records matching the specified domain", func() {
						It("returns an empty set of records", func() {
							records, err := recordSet.Filter([]string{"my-instance.my-group.my-network.my-deployment.wrong-domain."}, false)
							Expect(err).NotTo(HaveOccurred())

							Expect(records).To(HaveLen(0))
						})
					})
				})
			})
		})
	})

	Context("when resolving aliases", func() {
		BeforeEach(func() {
			aliasList = mustNewConfigFromMap(map[string][]string{
				"alias1":              {"q-s0.my-group.my-network.my-deployment.a1_domain1.", "q-s0.my-group.my-network.my-deployment.a1_domain2."},
				"alias2":              {"q-s0.my-group.my-network.my-deployment.a2_domain1."},
				"ipalias":             {"5.5.5.5"},
				"_.alias2":            {"_.my-group.my-network.my-deployment.a2_domain1.", "_.my-group.my-network.my-deployment.b2_domain1."},
				"nonexistentalias":    {"q-&&&&&.my-group.my-network.my-deployment.b2_domain1.", "q-&&&&&.my-group.my-network.my-deployment.a2_domain1."},
				"aliaswithonefailure": {"q-s0.my-group.my-network.my-deployment.a1_domain1.", "q-s0.my-group.my-network.my-deployment.domaindoesntexist."},
			})

			jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "1.1.1.1", "a2_domain1", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "2.2.2.2", "b2_domain1", 2],
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "3.3.3.3", "a1_domain1", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "4.4.4.4", "a1_domain2", 2]
					]
				}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		Describe("expanding aliases", func() {
			It("expands aliases to hosts", func() {
				expandedAliases := recordSet.ExpandAliases("q-s0.alias2.")
				Expect(expandedAliases).To(Equal([]string{"q-s0.my-group.my-network.my-deployment.a2_domain1.",
					"q-s0.my-group.my-network.my-deployment.b2_domain1.",
				}))
			})
		})

		Describe("all records", func() {
			It("returns all records", func() {
				Expect(recordSet.AllRecords()).To(Equal(&recordSet.Records))
			})
		})

		Context("when the message contains a underscore style alias", func() {
			It("translates the question preserving the capture", func() {
				resolutions, err := recordSet.Resolve("q-s0.alias2.")

				Expect(err).ToNot(HaveOccurred())
				Expect(resolutions).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
			})

			It("returns a non successful return code when a resoution fails", func() {
				_, err := recordSet.Resolve("nonexistentalias.")

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failures occurred when resolving alias domains:")))
			})
		})

		Context("when resolving an aliased host", func() {
			It("resolves the alias", func() {
				resolutions, err := recordSet.Resolve("alias2.")

				Expect(err).ToNot(HaveOccurred())
				Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
			})

			Context("when alias points to an IP directly", func() {
				It("resolves the alias to the IP", func() {
					resolutions, err := recordSet.Resolve("ipalias.")

					Expect(err).ToNot(HaveOccurred())
					Expect(resolutions).To(Equal([]string{"5.5.5.5"}))
				})
			})

			Context("when alias resolves to multiple hosts", func() {
				It("resolves the alias to all underlying hosts", func() {
					resolutions, err := recordSet.Resolve("alias1.")

					Expect(err).ToNot(HaveOccurred())
					Expect(resolutions).To(Equal([]string{"3.3.3.3", "4.4.4.4"}))
				})

				Context("and a subset of the resolutions fails", func() {
					It("returns the ones that succeeded", func() {
						resolutions, err := recordSet.Resolve("aliaswithonefailure.")

						Expect(err).ToNot(HaveOccurred())
						Expect(resolutions).To(Equal([]string{"3.3.3.3"}))
					})
				})
			})
		})
	})

	Context("when health watching is enabled", func() {
		var subscriptionChan chan bool
		BeforeEach(func() {
			subscriptionChan = make(chan bool, 1)
			fileReader.SubscribeReturns(subscriptionChan)

			fakeHealthWatcher.IsHealthyStub = func(ip string) bool {
				switch ip {
				case "123.123.123.123":
					return true
				case "123.123.123.5":
					return true
				case "123.123.123.246":
					return false
				}
				return false
			}

			aliasList = mustNewConfigFromMap(
				map[string][]string{
					"alias1": {
						"q-s1.my-group.my-network.my-deployment.a1_domain1.",
						"q-s3.my-group.my-network.my-deployment.a1_domain2.",
					},
				})

			jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.246", "my-domain", 2],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "246.246.246.246", "a1_domain1", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "246.246.246.247", "a1_domain2", 2]
					]
				}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
		})

		It("does not re-track already-tracked IPs", func() {
			ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(ConsistOf("123.123.123.123"))
		})

		Context("when an alias is supplied", func() {
			BeforeEach(func() {
				fakeHealthWatcher.IsHealthyStub = func(ip string) bool {
					switch ip {
					case "246.246.246.246":
						return false
					case "246.246.246.247":
						return true
					}
					return false
				}

				aliasList = mustNewConfigFromMap(
					map[string][]string{
						"alias1": {
							"q-s1.my-group.my-network.my-deployment.a1_domain1.",
							"q-s3.my-group.my-network.my-deployment.a1_domain2.",
						},
					})

				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "246.246.246.246", "a1_domain1", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "246.246.246.247", "a1_domain2", 2]
					]
				}`)
				fileReader.GetReturns(jsonBytes, nil)
				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the strategies are mixed", func() {
				It("returns the proper records", func() {
					ips, err := recordSet.Resolve("alias1.")
					Expect(err).NotTo(HaveOccurred())
					Expect(ips).To(ConsistOf("246.246.246.246", "246.246.246.247"))
				})
			})
		})

		Context("when the 'smart' strategy is specified", func() {
			It("returns only the healthy ips", func() {
				ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.123"))
				Eventually(fakeHealthWatcher.TrackCallCount).Should(Equal(2))
				Expect(fakeHealthWatcher.TrackArgsForCall(0)).To(Equal("123.123.123.123"))
				Expect(fakeHealthWatcher.TrackArgsForCall(1)).To(Equal("123.123.123.246"))

				ips, err = recordSet.Resolve("q-s3.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.123"))
				Eventually(fakeHealthWatcher.TrackCallCount).Should(Equal(4))
				Expect(fakeHealthWatcher.TrackArgsForCall(2)).To(Equal("123.123.123.123"))
				Expect(fakeHealthWatcher.TrackArgsForCall(3)).To(Equal("123.123.123.246"))
			})

			Context("when all ips are un-healthy", func() {
				BeforeEach(func() {
					fakeHealthWatcher.IsHealthyReturns(false)
				})

				It("returns all ips", func() {
					ips, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
					Expect(err).NotTo(HaveOccurred())
					Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
				})
			})
		})

		Context("when 'unhealthy' strategy is specified", func() {
			It("returns only unhealthy records", func() {
				ips, err := recordSet.Resolve("q-s1.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.246"))
			})
		})

		Context("when 'healthy' strategy is specified", func() {
			It("returns only the healthy records", func() {
				ips, err := recordSet.Resolve("q-s3.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.123"))
			})
		})

		Context("when 'all' strategy is specified", func() {
			It("returns all of the records regardless of health", func() {
				ips, err := recordSet.Resolve("q-s4.my-group.my-network.my-deployment.my-domain.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
			})
		})

		Context("when the ips under a tracked domain change", func() {
			BeforeEach(func() {
				recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				Eventually(fakeHealthWatcher.IsHealthyCallCount).Should(Equal(2))
				Expect(fakeHealthWatcher.IsHealthyArgsForCall(0)).To(Equal("123.123.123.123"))
				Expect(fakeHealthWatcher.IsHealthyArgsForCall(1)).To(Equal("123.123.123.246"))
				Eventually(fakeHealthWatcher.TrackCallCount).Should(Equal(2))
				Expect(fakeHealthWatcher.TrackArgsForCall(0)).To(Equal("123.123.123.123"))
				Expect(fakeHealthWatcher.TrackArgsForCall(1)).To(Equal("123.123.123.246"))

				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.5", "my-domain", 2]
					]
				}`)

				fileReader.GetReturns(jsonBytes, nil)

				subscriptionChan <- true
			})

			It("returns the new ones", func() {
				Eventually(func() ([]string, error) {
					return recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
				}).Should(ConsistOf("123.123.123.123", "123.123.123.5"))
			})

			It("checks the health of new ones", func() {
				Eventually(fakeHealthWatcher.TrackCallCount).Should(Equal(3))
				Eventually(fakeHealthWatcher.TrackArgsForCall(2)).Should(Equal("123.123.123.5"))
			})

			It("stops tracking old ones", func() {
				Eventually(fakeHealthWatcher.UntrackCallCount).Should(Equal(1))
				Expect(fakeHealthWatcher.UntrackArgsForCall(0)).To(Equal("123.123.123.246"))
			})
		})

		Context("when the ips not under a tracked domain change", func() {
			Describe("limiting tracked domains", func() {
				BeforeEach(func() {
					fakeHealthWatcher.IsHealthyReturns(true)

					jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "my-domain", 2],
						["instance2", "2", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.125", "my-domain", 3],
						["instance3", "3", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "123.123.123.126", "my-domain", 4],
						["instance4", "4", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.127", "my-domain", 5],
						["instance5", "5", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.128", "my-domain", 6],
						["instance6", "6", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.129", "my-domain", 7]
					]
				}`)

					fileReader.GetReturns(jsonBytes, nil)

					var err error
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger)

					Expect(err).ToNot(HaveOccurred())
				})

				It("tracks no more than the maximum number of domains (5) domains", func() {
					for i := 1; i <= 7; i++ {
						recordSet.Resolve(fmt.Sprintf("q-i%d.my-group.my-network.my-deployment.my-domain.", i))
					}

					Eventually(fakeHealthWatcher.UntrackCallCount).Should(Equal(2))

					Expect([]string{
						fakeHealthWatcher.UntrackArgsForCall(0),
						fakeHealthWatcher.UntrackArgsForCall(1),
					}).To(ConsistOf(
						"123.123.123.123",
						"123.123.123.124",
					))
				})
			})
		})

		Context("when filtering", func() {
			It("does not track additional IPs", func() {
				ips, err := recordSet.Filter([]string{"q-s0.my-group.my-network.my-deployment.my-domain."}, false)
				Consistently(fakeHealthWatcher.TrackCallCount).Should(Equal(0))
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf([]record.Record{
					{
						ID:            "instance0",
						NumID:         "0",
						Group:         "my-group",
						GroupIDs:      []string{"1"},
						Network:       "my-network",
						NetworkID:     "1",
						Deployment:    "my-deployment",
						IP:            "123.123.123.123",
						Domain:        "my-domain.",
						AZ:            "az1",
						AZID:          "1",
						InstanceIndex: "1",
					}}))
			})
		})
	})
})

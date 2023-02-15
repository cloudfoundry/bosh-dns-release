package records_test

import (
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
	"bosh-dns/healthcheck/api"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func dereferencer(r []record.Record) []record.Record {
	out := []record.Record{}
	out = append(out, r...)

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
		recordSet             *records.RecordSet
		fakeLogger            *fakes.FakeLogger
		fileReader            *recordsfakes.FakeFileReader
		aliasList             aliases.Config
		shutdownChan          chan struct{}
		fakeHealthWatcher     *healthinessfakes.FakeHealthWatcher
		fakeQueryFilterer     *recordsfakes.FakeFilterer
		fakeHealthFilterer    *recordsfakes.FakeFilterer
		fakeFiltererFactory   *recordsfakes.FakeFiltererFactory
		fakeAliasQueryEncoder *recordsfakes.FakeAliasQueryEncoder
	)

	BeforeEach(func() {
		fakeLogger = &fakes.FakeLogger{}
		fileReader = &recordsfakes.FakeFileReader{}
		fakeQueryFilterer = &recordsfakes.FakeFilterer{}
		fakeHealthFilterer = &recordsfakes.FakeFilterer{}
		fakeFiltererFactory = &recordsfakes.FakeFiltererFactory{}
		fakeFiltererFactory.NewQueryFiltererReturns(fakeQueryFilterer)
		fakeFiltererFactory.NewHealthFiltererReturns(fakeHealthFilterer)
		fakeAliasQueryEncoder = &recordsfakes.FakeAliasQueryEncoder{}

		aliasList = mustNewConfigFromMap(map[string][]string{})
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		shutdownChan = make(chan struct{})
		fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: api.StatusRunning})

		fakeQueryFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
			return recs
		}
	})

	AfterEach(func() {
		close(shutdownChan)
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
			})

			It("parses the instance index", func() {
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.domain."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

				Expect(err).ToNot(HaveOccurred())
			})

			It("normalizes domain names", func() {
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recordSet.Domains()).To(ConsistOf("withadot.", "nodot.", "domain."))
				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
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

	Describe("all records", func() {
		BeforeEach(func() {
			jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "1.1.1.1", "a2_domain1", 1],
						["instance1", "1", "my-group", ["1"], "az2", "2", "my-network", "1", "my-deployment", "2.2.2.2", "b2_domain1", 2]
					]
				}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

			Expect(err).ToNot(HaveOccurred())
		})

		It("returns all records", func() {
			all := recordSet.AllRecords()
			Expect(all).To(HaveLen(2))
			Expect(all[0].ID).To(Equal("instance0"))
			Expect(all[1].ID).To(Equal("instance1"))
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
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
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
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
			Expect(err).ToNot(HaveOccurred())

			Expect(recordSet.HasIP("123.123.123.123")).To(Equal(true))
			Expect(recordSet.HasIP("127.0.0.1")).To(Equal(false))
		})
	})

	Describe("GetFQDNs", func() {
		BeforeEach(func() {
			aliasList = mustNewConfigFromMap(map[string][]string{
				"alias1": {"instance0.my-group.my-network.my-deployment.withadot"},
				"alias2": {"123.123.123.124"},
			})
		})

		It("returns fqdns if an IP is known", func() {
			jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", "my-group", "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "withadot."],
					["instance1", "1", "my-group", "az2", "2", "my-network", "1", "my-deployment", "123.123.123.124", "nodot"],
					["instance2", "2", "my-group", "az3", null, "my-network", "1", "my-deployment", "123.123.123.125", "domain."]
				],
				"records": [
					["123.123.123.123", "instance0.my-group.my-network.my-deployment.withadot"],
					["123.123.123.123", "0.my-group.my-network.my-deployment.withadot"],
					["123.123.123.124", "instance1.my-group.my-network.my-deployment.nodot"],
					["123.123.123.124", "1.my-group.my-network.my-deployment.nodot"],
					["123.123.123.125", "instance2.my-group.my-network.my-deployment.domain"],
					["123.123.123.125", "2.my-group.my-network.my-deployment.domain"]
				]
			}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
			Expect(err).ToNot(HaveOccurred())

			Expect(recordSet.GetFQDNs("123.123.123.123")).To(ConsistOf("alias1.", "instance0.my-group.my-network.my-deployment.withadot.", "0.my-group.my-network.my-deployment.withadot."))
			Expect(recordSet.GetFQDNs("123.123.123.124")).To(ConsistOf("instance1.my-group.my-network.my-deployment.nodot.", "1.my-group.my-network.my-deployment.nodot.", "alias2."))
			Expect(recordSet.GetFQDNs("123.123.123.125")).To(ConsistOf("instance2.my-group.my-network.my-deployment.domain.", "2.my-group.my-network.my-deployment.domain."))
			Expect(recordSet.GetFQDNs("127.0.0.1")).To(BeEmpty())
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

			aliasList = mustNewConfigFromMap(map[string][]string{
				"analias.": []string{"*.my-group.my-network.my-deployment.bosh."},
			})

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
			Expect(err).ToNot(HaveOccurred())

			_, err = recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
			Expect(err).NotTo(HaveOccurred())

			_, recs := fakeHealthFilterer.FilterArgsForCall(0)
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].IP).To(Equal("123.123.123.123"))

			_, shouldTrack := fakeFiltererFactory.NewHealthFiltererArgsForCall(0)
			Expect(shouldTrack).To(BeTrue())

			Expect(err).NotTo(HaveOccurred())
		})

		Context("when updating to valid json", func() {
			var (
				subscribers []<-chan bool
			)

			BeforeEach(func() {
				jsonBytes := []byte(`{
				"record_keys": ["id", "num_id", "group_ids", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["instance0", "0", ["2"], "my-group", "az1", "1", "my-network", "1", "my-deployment", "234.234.234.234", "bosh2."]
				],
				"aliases": {
				  "foodomain.bar.": [
						{
							"group_id": "2",
							"root_domain": "bosh"
						}
					]
				}
			}`)
				fileReader.GetReturns(jsonBytes, nil)
				subscriptionChan <- true
				subscribers = append(subscribers, recordSet.Subscribe())
				subscribers = append(subscribers, recordSet.Subscribe())
			})

			It("updates its set of records", func() {
				Eventually(func() []string {
					_, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh2.")
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeHealthFilterer.FilterCallCount()).To(BeNumerically(">", 0))
					_, recs := fakeHealthFilterer.FilterArgsForCall(fakeHealthFilterer.FilterCallCount() - 1)
					ips := []string{}
					for _, r := range recs {
						ips = append(ips, r.IP)
					}
					return ips
				}).Should(Equal([]string{"234.234.234.234"}))

				Eventually(func() []string {
					_, err := recordSet.Resolve("foodomain.bar.")
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeHealthFilterer.FilterCallCount()).To(BeNumerically(">", 0))
					_, recs := fakeHealthFilterer.FilterArgsForCall(fakeHealthFilterer.FilterCallCount() - 1)
					ips := []string{}
					for _, r := range recs {
						ips = append(ips, r.IP)
					}
					return ips
				}).Should(Equal([]string{"234.234.234.234"}))
			})

			It("Updates its set of domains", func() {
				Eventually(recordSet.Domains).Should(Equal([]string{"bosh2.", "analias."}))
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
					_, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeHealthFilterer.FilterCallCount()).To(BeNumerically(">", 0))
					_, recs := fakeHealthFilterer.FilterArgsForCall(fakeHealthFilterer.FilterCallCount() - 1)

					ips := []string{}
					for _, r := range recs {
						ips = append(ips, r.IP)
					}
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
					_, err := recordSet.Resolve("instance0.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeHealthFilterer.FilterCallCount()).To(BeNumerically(">", 0))
					_, recs := fakeHealthFilterer.FilterArgsForCall(fakeHealthFilterer.FilterCallCount() - 1)

					ips := []string{}
					for _, r := range recs {
						ips = append(ips, r.IP)
					}

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
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
					Expect(err).ToNot(HaveOccurred())

					_, err = recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())
					_, shouldTrack := fakeFiltererFactory.NewHealthFiltererArgsForCall(0)
					Expect(shouldTrack).To(BeTrue())
					_, recs := fakeHealthFilterer.FilterArgsForCall(0)
					Expect(recs).To(HaveLen(1))
					Expect(recs[0].IP).To(Equal("123.123.123.123"))

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
					recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

					Expect(err).ToNot(HaveOccurred())
				})

				It("does not blow up, logs the invalid record, and returns the info that was parsed correctly", func() {
					_, err := recordSet.Resolve("q-s0.my-group.my-network.my-deployment.my-domain.")
					Expect(err).ToNot(HaveOccurred())

					_, shouldTrack := fakeFiltererFactory.NewHealthFiltererArgsForCall(0)
					Expect(shouldTrack).To(BeTrue())
					_, recs := fakeHealthFilterer.FilterArgsForCall(0)

					Expect(recs).To(HaveLen(2))
					Expect(recs[0].IP).To(Equal("123.123.123.123"))
					Expect(recs[1].IP).To(Equal("123.123.123.126"))

					Expect(fakeLogger.WarnCallCount()).To(Equal(1))
					logTag, _, rest := fakeLogger.WarnArgsForCall(0)
					Expect(logTag).To(Equal("RecordSet"))
					Expect(rest[0]).To(Equal(10))
					Expect(rest[1]).To(Equal(9))
					Expect(rest[2]).To(Equal(1))
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())

				_, err = recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true)
				Expect(err).To(MatchError(records.DomainError))

				_, recs := fakeQueryFilterer.FilterArgsForCall(0)
				Expect(recs).To(BeEmpty())
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).NotTo(HaveOccurred())

				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).ToNot(BeEmpty())
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())

				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).NotTo(BeEmpty())

				Expect(recs[0].AZID).To(Equal(""))
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())

				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).NotTo(BeEmpty())
				Expect(recs[0].InstanceIndex).To(Equal(""))
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
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
				recordSet.ResolveRecords([]string{"dummy.my-group.my-network.my-deployment.bosh."}, true) //nolint:errcheck
				_, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(recs).NotTo(BeEmpty())
				Expect(recs[0].GroupIDs).To(BeEmpty())
			})
		})
	})

	Describe("Resolve", func() {
		JustBeforeEach(func() {
			jsonBytes := []byte(`{
									"record_keys": ["id", "instance_group", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
									"record_infos": [
										["instance1", "my-group", "az2", "1", "my-network", "my-deployment", "123.123.123.124", "domain.", 1]
									]
								}`)
			fileReader.GetReturns(jsonBytes, nil)

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when fqdn is already an IP address", func() {
			It("return the IP back", func() {
				records, err := recordSet.Resolve("123.123.123.123")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(ContainElement("123.123.123.123"))
			})
		})

		Context("when no records match and no aliases are IP addresses", func() {
			BeforeEach(func() {
				fakeQueryFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
					return []record.Record{}
				}
			})
			It("returns DomainError", func() {
				_, err := recordSet.Resolve("q-s0m1.my-group2.my-network2.my-deployment2.domain.")
				Expect(err).To(MatchError(records.DomainError))
			})
		})

		Context("when no records match but fqdn aliased to an IP address", func() {
			BeforeEach(func() {
				aliasList = mustNewConfigFromMap(map[string][]string{
					"alias.my.": []string{"1.2.3.4"},
				})
				fakeQueryFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
					return []record.Record{}
				}
			})
			It("returns the IP", func() {
				records, err := recordSet.Resolve("alias.my.")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(ContainElement("1.2.3.4"))
			})
		})
	})

	Describe("ResolveRecords", func() {
		Context("when there are records matching the query based fqdn", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1]
					]
				}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
			})

			It("it filters via the Filterer with criteria", func() {
				recordSet.ResolveRecords([]string{"q-s0m1.my-group.my-network.my-deployment.my-domain."}, true) //nolint:errcheck
				crit, recs := fakeHealthFilterer.FilterArgsForCall(0)

				Expect(crit).To(Equal(criteria.Criteria{
					"s":                 []string{"0"},
					"m":                 []string{"1"},
					"instanceGroupName": []string{"my-group"},
					"network":           []string{"my-network"},
					"deployment":        []string{"my-deployment"},
					"domain":            []string{"my-domain."},
					"fqdn":              []string{"q-s0m1.my-group.my-network.my-deployment.my-domain."},
				}))
				Expect(recs).To(WithTransform(dereferencer, ContainElement(record.Record{
					ID:            "instance0",
					NumID:         "0",
					Group:         "my-group",
					GroupIDs:      []string{"1"},
					NetworkID:     "1",
					Network:       "my-network",
					Deployment:    "my-deployment",
					IP:            "123.123.123.123",
					Domain:        "my-domain.",
					AZ:            "az1",
					AZID:          "1",
					InstanceIndex: "1",
				})))
			})
		})

		Context("when there are no records matching the domain", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": []
				}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
			})

			It("it returns DomainError", func() {
				_, err := recordSet.ResolveRecords([]string{"q-s0m1.my-group.my-network.my-deployment.my-domain."}, true)
				Expect(err).To(MatchError(records.DomainError))
			})
		})

		Context("when there are records matching the domain but not the query filter", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1]
					]
				}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
			})

			It("it returns empty records", func() {
				recs, err := recordSet.ResolveRecords([]string{"q-s1m1.my-group.my-network.my-deployment.my-domain."}, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(recs).To(BeEmpty())
			})
		})

		Context("when query can't be parsed", func() {
			BeforeEach(func() {
				jsonBytes := []byte(`{
					"record_keys":
						["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain", "instance_index"],
					"record_infos": [
						["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "my-domain", 1]
					]
				}`)
				fileReader.GetReturns(jsonBytes, nil)

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)
				Expect(err).ToNot(HaveOccurred())
			})

			It("it returns CriteriaError", func() {
				_, err := recordSet.ResolveRecords([]string{"notaquery"}, true)
				Expect(err).To(MatchError(records.CriteriaError))
			})
		})
	})

	Context("aliases", func() {
		Context("when the aliases are provided are seeded", func() {
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

				fakeHealthFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
					crit := mm.(criteria.Criteria)

					switch crit["fqdn"][0] {
					case "q-s0.my-group.my-network.my-deployment.a1_domain1.":
						return []record.Record{recs[2]}
					case "q-s0.my-group.my-network.my-deployment.a1_domain2.":
						return []record.Record{recs[3]}
					case "q-s0.my-group.my-network.my-deployment.a2_domain1.":
						return []record.Record{recs[0]}
					case "q-s0.my-group.my-network.my-deployment.b2_domain1.":
						return []record.Record{recs[1]}
					}
					return []record.Record{}
				}

				var err error
				recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

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

			Context("when the message contains a underscore style alias", func() {
				It("translates the question preserving the capture", func() {
					resolutions, err := recordSet.Resolve("q-s0.alias2.")

					Expect(err).ToNot(HaveOccurred())
					Expect(resolutions).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
				})

				It("returns a non successful return code when a resolution fails", func() {
					_, err := recordSet.Resolve("nonexistentalias.")

					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(records.CriteriaError))
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

				Context("when aliases are merged from multiple sources", func() {
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
					],
					"aliases": {
						"globalalias": [{
							"group_id": "1",
							"root_domain": "a2_domain1"
						}]
					}
				}`)
						fileReader.GetReturns(jsonBytes, nil)
						fakeAliasQueryEncoder.EncodeAliasesIntoQueriesReturns(
							map[string][]string{
								"globalalias":   {"q-s0.my-group.my-network.my-deployment.a2_domain1."},
								"filteredalias": {"q-s0.q-g1.a2_domain1."},
							},
						)

						fakeHealthFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
							crit := mm.(criteria.Criteria)

							switch crit["fqdn"][0] {
							case "q-s0.my-group.my-network.my-deployment.a1_domain1.":
								return []record.Record{recs[2]}
							case "q-s0.my-group.my-network.my-deployment.a1_domain2.":
								return []record.Record{recs[3]}
							case "q-s0.my-group.my-network.my-deployment.a2_domain1.":
								return []record.Record{recs[0]}
							case "q-s0.my-group.my-network.my-deployment.b2_domain1.":
								return []record.Record{recs[1]}
							case "q-s0.q-g1.a2_domain1.":
								return []record.Record{recs[0]}
							}
							return []record.Record{}
						}

						var err error
						recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

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

					Context("when the message contains a underscore style alias", func() {
						It("translates the question preserving the capture", func() {
							resolutions, err := recordSet.Resolve("q-s0.alias2.")

							Expect(err).ToNot(HaveOccurred())
							Expect(resolutions).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
						})

						It("returns a non successful return code when a resolution fails", func() {
							_, err := recordSet.Resolve("nonexistentalias.")

							Expect(err).To(HaveOccurred())
							Expect(err).To(MatchError(records.CriteriaError))
						})
					})

					Context("when resolving an aliased host", func() {
						It("resolves the alias", func() {
							resolutions, err := recordSet.Resolve("alias2.")

							Expect(err).ToNot(HaveOccurred())
							Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
						})

						Context("when the alias is global", func() {
							It("resolves the alias", func() {
								resolutions, err := recordSet.Resolve("globalalias.")

								Expect(err).ToNot(HaveOccurred())
								Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
							})
						})

						Context("when resolving aliases", func() {
							JustBeforeEach(func() {
								fakeHealthFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
									crit := mm.(criteria.Criteria)

									switch crit["fqdn"][0] {
									case "q-s0.q-g1.a2_domain1.":
										return []record.Record{recs[0]}
									}
									return []record.Record{}
								}
							})

							It("includes default values", func() {
								resolutions, err := recordSet.Resolve("filteredalias.")

								Expect(err).ToNot(HaveOccurred())
								Expect(resolutions).To(Equal([]string{"1.1.1.1"}))
							})
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
			})
		})
	})

	Context("Handling parallel reads and writes", func() {
		var (
			subscriptionChan chan bool
		)

		BeforeEach(func() {
			subscriptionChan = make(chan bool, 1)
			fileReader.SubscribeReturns(subscriptionChan)
			for i := 0; i < 10; i++ {
				jsonBytes := []byte(fmt.Sprintf(`{
									"record_keys": ["id", "instance_group", "group_ids", "az", "az_id", "network", "deployment", "ip", "domain", "instance_index"],
									"record_infos": [
										["instance0", "my-group", ["1"], "az1", "1", "my-network", "my-deployment", "123.123.123.123", "domain.", 0],
										["instance1", "my-group", ["1"], "az2", "1", "my-network", "my-deployment", "123.123.123.124", "domain.", 1]
									],
									"aliases": {
										"a1.internal.": [
											{
												"group_id": "1",
												"root_domain": "domain"
											}
										]
									},
									"Version": %d,
									"records": [
										["123.123.123.123", "instance0.my-group.my-network.my-deployment.domain."],
										["123.123.123.124", "instance1.my-group.my-network.my-deployment.domain."]
									]
								}`, i))
				fileReader.GetReturnsOnCall(i, jsonBytes, nil)
			}

			aliasList = mustNewConfigFromMap(map[string][]string{
				"a1.internal.": []string{"*.my-group.my-network.my-deployment.domain."},
			})

			fakeHealthFilterer.FilterStub = func(mm criteria.MatchMaker, recs []record.Record) []record.Record {
				return recs
			}

			var err error
			recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, fakeFiltererFactory, fakeAliasQueryEncoder)

			Expect(err).ToNot(HaveOccurred())
		})

		It("does not deadlock", func(ctx SpecContext) {
			wg := sync.WaitGroup{}

			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						records := recordSet.AllRecords()
						Expect(records).To(HaveLen(2))
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						domains := recordSet.Domains()
						Expect(domains).To(ConsistOf("domain.", "a1.internal."))
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						domains := recordSet.ExpandAliases("a1.internal.")
						Expect(domains).To(ConsistOf("q-s0.my-group.my-network.my-deployment.domain."))
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						domains := recordSet.GetFQDNs("123.123.123.123")
						Expect(domains).To(ConsistOf("instance0.my-group.my-network.my-deployment.domain."))
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						present := recordSet.HasIP("123.123.123.123")
						Expect(present).To(BeTrue())
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						ips, err := recordSet.Resolve("a1.internal.")
						Expect(err).ToNot(HaveOccurred())
						Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.124"))
					}
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					for j := 0; j < 10; j++ {
						records, err := recordSet.ResolveRecords([]string{"q-s0.my-group.my-network.my-deployment.domain."}, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(records).To(HaveLen(2))
					}
					wg.Done()
				}()
			}
			wg.Add(1)
			go func() {
				for i := 0; i < 10; i++ {
					subscriptionChan <- true
				}
				wg.Done()
			}()
			wg.Wait()
			// No Expect() block; instead, if this test fails, it generates a Ginkgo error "A running node failed to exit in time"
			// followed by voluminous output.
		}, NodeTimeout(10*time.Second))
	})
})

package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/config"
	"bosh-dns/dns/config/configfakes"
	"bosh-dns/dns/config/handlers"
	"bosh-dns/dns/manager/managerfakes"
	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
)

func TestSystemdResolvedUpdater(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemdResolvedUpdater Suite")
}

func mustNewConfigFromMap(load map[string][]string) aliases.Config {
	config, err := aliases.NewConfigFromMap(load)
	if err != nil {
		Fail(err.Error())
	}
	return config
}

func mustNewSuffixesFromMap(load map[string]string) aliases.Suffixes {
	suffixes, errs := aliases.NewSuffixesFromMap(load)
	if len(errs) > 0 {
		Fail(fmt.Sprintf("%v", errs))
	}
	return suffixes
}

var _ = Describe("parentDomains", func() {
	It("extracts the parent domain (drops the leftmost label) from each upcheck domain", func() {
		Expect(parentDomains([]string{"upcheck.bosh-dns.", "instance.health.bosh."})).To(ConsistOf("bosh-dns.", "health.bosh."))
	})

	It("handles multiple upcheck domains", func() {
		Expect(parentDomains([]string{"upcheck.bosh-dns.", "health.bosh."})).To(ConsistOf("bosh-dns.", "bosh."))
	})

	It("ignores domains with no dot or a trailing dot only", func() {
		Expect(parentDomains([]string{"bosh-dns.", "bosh-dns"})).To(BeEmpty())
	})

	It("returns empty when given no domains", func() {
		Expect(parentDomains([]string{})).To(BeEmpty())
	})
})

var _ = Describe("systemd-resolved domains", func() {
	var (
		shutdown       chan struct{}
		handlerDomains []string
		upcheckDomains []string
		recordsJSON    string
	)

	newRecordSet := func(aliasList aliases.Config) *records.RecordSet {
		fileReader := &recordsfakes.FakeFileReader{}
		fileReader.GetReturns([]byte(recordsJSON), nil)
		healthWatcher := healthiness.NewNopHealthWatcher()

		recordSet, err := records.NewRecordSet(
			fileReader,
			aliasList,
			healthWatcher,
			uint(5),
			shutdown,
			boshlog.NewLogger(boshlog.LevelNone),
			records.NewHealthFiltererFactory(healthWatcher, time.Second),
			records.NewAliasEncoder(),
		)
		Expect(err).ToNot(HaveOccurred())

		return recordSet
	}

	masterDomains := func(handlerDomains []string, recordSet *records.RecordSet, upcheckDomains []string) []string {
		var domains []string
		domains = append(domains, handlerDomains...)
		domains = append(domains, recordSet.Domains()...)
		domains = append(domains, parentDomains(upcheckDomains)...)
		return domains
	}

	BeforeEach(func() {
		shutdown = make(chan struct{})
		handlerDomains = []string{"corp.intranet.local.", "bosh."}
		upcheckDomains = []string{"upcheck.bosh-dns.", "health.bosh."}
		recordsJSON = `{
			"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
			"record_infos": [
				["instance0", "0", "my-group", ["1"], "az1", "1", "my-network", "1", "my-deployment", "123.123.123.123", "bosh."]
			],
			"aliases": {
				"_.my-group.bosh": [{"group_id": "1", "root_domain": "bosh", "placeholder_type": "uuid"}]
			}
		}`
	})

	AfterEach(func() {
		close(shutdown)
	})

	Describe("systemdResolvedDomains", func() {
		Context("when no suffix applies and no routing domains are configured", func() {
			var aliasList aliases.Config

			BeforeEach(func() {
				aliasList = mustNewConfigFromMap(map[string][]string{
					"Alias.One":       {"q-s0.one.default.cf.bosh"},
					"child.alias.one": {"q-s0.child.default.cf.bosh"},
					"_.under.alias":   {"_.domain"},
				})
			})

			It("sends the handler, record set, and upcheck parent domains unchanged and in order", func() {
				domains, registered := systemdResolvedDomains(handlerDomains, newRecordSet(aliasList), aliases.NewSuffixes(), upcheckDomains, nil)

				Expect(registered).To(BeFalse())
				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"bosh.",
					"alias.one.",
					"instance0.my-group.bosh.",
					"under.alias.",
					"bosh-dns.",
					"bosh.",
				}))
			})

			It("sends exactly the list built from Domains", func() {
				recordSet := newRecordSet(aliasList)
				expected := masterDomains(handlerDomains, recordSet, upcheckDomains)

				domains, _ := systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), upcheckDomains, nil)
				Expect(domains).To(Equal(expected))

				domains, _ = systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), upcheckDomains, []string{})
				Expect(domains).To(Equal(expected))
			})

			It("sends exactly the list built from Domains when every suffix names an undefined alias", func() {
				recordSet := newRecordSet(aliasList)

				domains, registered := systemdResolvedDomains(handlerDomains, recordSet, mustNewSuffixesFromMap(map[string]string{
					"missing.alias.one": "alias.one",
					"under.alias":       "alias",
				}), upcheckDomains, nil)

				Expect(registered).To(BeFalse())
				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
			})

			It("returns nothing when there are no domains", func() {
				recordsJSON = `{}`

				domains, registered := systemdResolvedDomains(nil, newRecordSet(aliases.NewConfig()), aliases.NewSuffixes(), nil, nil)

				Expect(registered).To(BeFalse())
				Expect(domains).To(BeEmpty())
			})
		})

		Context("when routing domains are configured", func() {
			It("replaces every name that equals or sits under a routing domain and drops the duplicates", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(map[string][]string{
					"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
					"b.service.cf.internal": {"q-s0.b.default.cf.bosh"},
				}))

				domains, registered := systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), upcheckDomains, []string{"bosh.", "cf.internal."})

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"cf.internal.",
					"bosh-dns.",
				}))
			})

			DescribeTable("uses the longest routing domain a name sits under, whatever the order",
				func(routingDomains []string) {
					recordSet := newRecordSet(mustNewConfigFromMap(map[string][]string{
						"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
						"b.other.cf.internal":   {"q-s0.b.default.cf.bosh"},
					}))

					domains, registered := systemdResolvedDomains(nil, recordSet, aliases.NewSuffixes(), nil, routingDomains)

					Expect(registered).To(BeTrue())
					Expect(domains).To(Equal([]string{
						"bosh.",
						"service.cf.internal.",
						"internal.",
						"instance0.my-group.bosh.",
					}))
				},
				Entry("shorter first", []string{"internal.", "service.cf.internal."}),
				Entry("longer first", []string{"service.cf.internal.", "internal."}),
			)

			It("sends both of two nested routing domains when each covers a name the other does not", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(map[string][]string{
					"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
					"b.other.cf.internal":   {"q-s0.b.default.cf.bosh"},
				}))

				domains, registered := systemdResolvedDomains(nil, recordSet, aliases.NewSuffixes(), nil, []string{"cf.internal.", "service.cf.internal."})

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"bosh.",
					"service.cf.internal.",
					"cf.internal.",
					"instance0.my-group.bosh.",
				}))
			})

			It("matches on label boundaries only", func() {
				domains, registered := systemdResolvedDomains([]string{"xbosh.", "bosh-dns."}, newRecordSet(aliases.NewConfig()), aliases.NewSuffixes(), nil, []string{"bosh."})

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"xbosh.",
					"bosh-dns.",
					"bosh.",
				}))
			})

			It("matches names regardless of case or a missing trailing dot and sends unmatched names unchanged", func() {
				domains, _ := systemdResolvedDomains([]string{"Corp.Intranet.Local", "Health.BOSH"}, newRecordSet(aliases.NewConfig()), aliases.NewSuffixes(), nil, []string{"bosh."})

				Expect(domains).To(Equal([]string{
					"Corp.Intranet.Local",
					"bosh.",
				}))
			})

			It("does not send or report a routing domain that covers none of the names", func() {
				domains, registered := systemdResolvedDomains(nil, newRecordSet(aliases.NewConfig()), aliases.NewSuffixes(), upcheckDomains, []string{"service.cf.internal."})

				Expect(registered).To(BeFalse())
				Expect(domains).To(Equal([]string{
					"bosh.",
					"instance0.my-group.bosh.",
					"bosh-dns.",
				}))
			})

			It("does not change Domains", func() {
				recordSet := newRecordSet(aliases.NewConfig())
				domains := recordSet.Domains()

				systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), upcheckDomains, []string{"bosh."})

				Expect(recordSet.Domains()).To(Equal(domains))
				Expect(domains).To(Equal([]string{"bosh.", "instance0.my-group.bosh."}))
			})
		})

		Context("when a suffix applies and no routing domains are configured", func() {
			It("sends the suffix in place of the aliases and drops the duplicates", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(map[string][]string{
					"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
					"b.service.cf.internal": {"q-s0.b.default.cf.bosh"},
				}))

				domains, registered := systemdResolvedDomains(handlerDomains, recordSet, mustNewSuffixesFromMap(map[string]string{
					"a.service.cf.internal": "service.cf.internal",
					"b.service.cf.internal": "service.cf.internal",
				}), upcheckDomains, nil)

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"instance0.my-group.bosh.",
					"service.cf.internal.",
					"bosh-dns.",
				}))
			})
		})

		Context("when a suffix applies and routing domains are configured", func() {
			It("replaces a suffix that sits under a routing domain with the routing domain", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(map[string][]string{
					"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
					"b.other.cf.internal":   {"q-s0.b.default.cf.bosh"},
				}))

				domains, registered := systemdResolvedDomains(nil, recordSet, mustNewSuffixesFromMap(map[string]string{
					"a.service.cf.internal": "service.cf.internal",
				}), upcheckDomains, []string{"cf.internal."})

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"bosh.",
					"cf.internal.",
					"instance0.my-group.bosh.",
					"bosh-dns.",
				}))
			})
		})

		Context("with the shape of a large cf deployment", func() {
			var (
				serviceAliases  map[string][]string
				serviceSuffixes map[string]string
			)

			BeforeEach(func() {
				recordsJSON = largeDeploymentRecordsJSON(600, 6)

				serviceAliases = map[string][]string{}
				serviceSuffixes = map[string]string{}
				for i := 0; i < 22; i++ {
					alias := fmt.Sprintf("service-%d.service.cf.internal", i)
					serviceAliases[alias] = []string{fmt.Sprintf("q-s0.service-%d.default.cf.bosh", i)}
					serviceSuffixes[alias] = "service.cf.internal"
				}

				handlerDomains = nil
				upcheckDomains = []string{"upcheck.bosh-dns."}
			})

			It("sends more than 256 domains without configuration", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(serviceAliases))

				domains, registered := systemdResolvedDomains(handlerDomains, recordSet, aliases.NewSuffixes(), upcheckDomains, nil)

				Expect(registered).To(BeFalse())
				Expect(domains).To(HaveLen(624))
				Expect(domains).To(Equal(append(recordSet.Domains(), "bosh-dns.")))
			})

			It("sends three domains with a bosh routing domain and a service suffix", func() {
				recordSet := newRecordSet(mustNewConfigFromMap(serviceAliases))

				domains, registered := systemdResolvedDomains(handlerDomains, recordSet, mustNewSuffixesFromMap(serviceSuffixes), upcheckDomains, []string{"bosh."})

				Expect(registered).To(BeTrue())
				Expect(domains).To(Equal([]string{
					"bosh.",
					"service.cf.internal.",
					"bosh-dns.",
				}))
				Expect(recordSet.Domains()).To(HaveLen(623))
			})
		})
	})

	Describe("domainsFromConfig", func() {
		const ownSuffixFile = "/var/vcap/jobs/bosh-dns/dns/alias_suffixes.json"

		var (
			cfg            config.Config
			fs             *boshsysfakes.FakeFileSystem
			fakeLogger     *loggerfakes.FakeLogger
			recordSet      *records.RecordSet
			recursorReader *configfakes.FakeRecursorReader
			suffixFiles    []string
		)

		warnings := func() []string {
			var messages []string
			for i := 0; i < fakeLogger.WarnCallCount(); i++ {
				_, message, args := fakeLogger.WarnArgsForCall(i)
				messages = append(messages, fmt.Sprintf(message, args...))
			}
			return messages
		}

		writeSuffixFile := func(job string, contents string) {
			file := fmt.Sprintf("/var/vcap/jobs/%s/dns/alias_suffixes.json", job)
			Expect(fs.WriteFileString(file, contents)).To(Succeed())
			suffixFiles = append(suffixFiles, file)
			fs.SetGlob("/var/vcap/jobs/*/dns/alias_suffixes.json", suffixFiles)
		}

		BeforeEach(func() {
			fs = boshsysfakes.NewFakeFileSystem()
			fakeLogger = &loggerfakes.FakeLogger{}
			recursorReader = &configfakes.FakeRecursorReader{}
			suffixFiles = nil

			cfg = config.NewDefaultConfig()
			cfg.ConfigureSystemdResolved = true
			cfg.DisableRecursors = true
			cfg.UpcheckDomains = upcheckDomains
			cfg.JobsDir = "/var/vcap/jobs"
			Expect(fs.WriteFileString(ownSuffixFile, `{}`)).To(Succeed())
		})

		JustBeforeEach(func() {
			recordSet = newRecordSet(mustNewConfigFromMap(map[string][]string{
				"a.service.cf.internal": {"q-s0.a.default.cf.bosh"},
				"b.service.cf.internal": {"q-s0.b.default.cf.bosh"},
			}))
		})

		Context("when nothing is configured", func() {
			It("sends exactly the list built from Domains and logs nothing", func() {
				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"bosh.",
					"a.service.cf.internal.",
					"b.service.cf.internal.",
					"instance0.my-group.bosh.",
					"bosh-dns.",
					"bosh.",
				}))
				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
				Expect(fakeLogger.WarnCallCount()).To(Equal(0))
				Expect(fakeLogger.InfoCallCount()).To(Equal(0))
			})

			It("does not read the recursors when bosh-dns would read them from resolv.conf", func() {
				cfg.DisableRecursors = false
				recursorReader.GetReturns(nil, errors.New("no resolv.conf"))

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
				Expect(recursorReader.GetCallCount()).To(Equal(0))
				Expect(fakeLogger.WarnCallCount()).To(Equal(0))
			})
		})

		Context("when routing domains and alias suffix files are configured", func() {
			BeforeEach(func() {
				cfg.SystemdResolvedRoutingDomains = []string{"BOSH"}
				writeSuffixFile("a", `{
					"a.service.cf.internal": "service.cf.internal",
					"b.service.cf.internal": "service.cf.internal"
				}`)
			})

			It("passes the canonical routing domains, the suffixes from the default glob under jobs_dir, and the upcheck domains through", func() {
				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"service.cf.internal.",
					"bosh-dns.",
				}))
				Expect(fakeLogger.WarnCallCount()).To(Equal(0))
			})

			It("uses the configured glob in place of the default", func() {
				cfg.AliasSuffixFilesGlob = "/other/*/alias_suffixes.json"

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"a.service.cf.internal.",
					"b.service.cf.internal.",
					"bosh-dns.",
				}))
			})

			It("logs and skips an alias suffix file it cannot read", func() {
				fs.RegisterReadFileError("/var/vcap/jobs/a/dns/alias_suffixes.json", errors.New("permission denied"))

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"a.service.cf.internal.",
					"b.service.cf.internal.",
					"bosh-dns.",
				}))
				Expect(warnings()).To(ConsistOf(
					"Ignoring alias suffix file /var/vcap/jobs/a/dns/alias_suffixes.json: could not be read: permission denied",
				))
			})

			It("lets the bosh-dns job's own alias_suffixes win over another job's file", func() {
				Expect(fs.WriteFileString("/var/vcap/jobs/a/dns/alias_suffixes.json", `{"a.service.cf.internal": "service.cf.internal"}`)).To(Succeed())
				Expect(fs.WriteFileString(ownSuffixFile, `{"a.service.cf.internal": "a.service.cf.internal"}`)).To(Succeed())

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"a.service.cf.internal.",
					"b.service.cf.internal.",
					"bosh-dns.",
				}))
				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
			})
		})

		It("logs and drops a routing domain that cannot be registered and keeps the others", func() {
			cfg.SystemdResolvedRoutingDomains = []string{"", ".", "~bosh", "*.bosh", "bo sh", "-bad.bosh", "Kube.internal", "service.cf.internal"}

			domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

			Expect(domains).To(Equal([]string{
				"corp.intranet.local.",
				"bosh.",
				"service.cf.internal.",
				"instance0.my-group.bosh.",
				"bosh-dns.",
			}))
			Expect(warnings()).To(Equal([]string{
				`Ignoring the systemd_resolved.routing_domains entry "": empty domain`,
				`Ignoring the systemd_resolved.routing_domains entry ".": the root domain is not allowed`,
				`Ignoring the systemd_resolved.routing_domains entry "~bosh": write it without the ~ prefix`,
				`Ignoring the systemd_resolved.routing_domains entry "*.bosh": wildcards are not allowed`,
				`Ignoring the systemd_resolved.routing_domains entry "bo sh": whitespace is not allowed`,
				`Ignoring the systemd_resolved.routing_domains entry "-bad.bosh": not a valid domain name`,
				`Ignoring the systemd_resolved.routing_domains entry "Kube.internal": only ASCII characters are allowed`,
			}))
		})

		It("covers the per-instance names of a uuid link alias outside bosh with a routing domain, which an alias suffix cannot do", func() {
			recordsJSON = `{
				"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": [
					["0a1b", "0", "cell", ["7"], "az1", "1", "default", "1", "cf", "10.0.0.1", "bosh."],
					["2c3d", "1", "cell", ["7"], "az1", "1", "default", "1", "cf", "10.0.0.2", "bosh."]
				],
				"aliases": {
					"_.cell.service.cf.internal": [{"group_id": "7", "root_domain": "bosh", "placeholder_type": "uuid"}]
				}
			}`
			recordSet = newRecordSet(aliases.NewConfig())
			writeSuffixFile("a", `{"_.cell.service.cf.internal": "service.cf.internal"}`)

			Expect(domainsFromConfig(cfg, ownSuffixFile, nil, recordSet, fs, recursorReader, fakeLogger)).To(Equal([]string{
				"bosh.",
				"0a1b.cell.service.cf.internal.",
				"2c3d.cell.service.cf.internal.",
				"bosh-dns.",
				"bosh.",
			}))

			cfg.SystemdResolvedRoutingDomains = []string{"service.cf.internal"}
			Expect(domainsFromConfig(cfg, ownSuffixFile, nil, recordSet, fs, recursorReader, fakeLogger)).To(Equal([]string{
				"bosh.",
				"service.cf.internal.",
				"bosh-dns.",
			}))
		})

		Context("when bosh-dns can forward to the systemd-resolved stub resolver", func() {
			BeforeEach(func() {
				cfg.DisableRecursors = false
				cfg.Recursors = nil
				cfg.SystemdResolvedRoutingDomains = []string{"bosh."}
				recursorReader.GetReturns([]string{"127.0.0.53:53"}, nil)
				writeSuffixFile("a", `{"a.service.cf.internal": "service.cf.internal"}`)
			})

			It("ignores the routing domains and alias suffixes with a warning that says why", func() {
				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
				Expect(warnings()).To(ConsistOf(And(
					ContainSubstring("Ignoring alias suffixes and systemd_resolved.routing_domains"),
					ContainSubstring("systemd-resolved stub resolver"),
					ContainSubstring("disable_recursors: true"),
				)))
			})

			It("does not warn when nothing would be registered from a suffix or a routing domain", func() {
				cfg.SystemdResolvedRoutingDomains = []string{"unused.internal."}
				suffixFiles = nil
				writeSuffixFile("b", `{"missing.service.cf.internal": "service.cf.internal"}`)

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
				Expect(fakeLogger.WarnCallCount()).To(Equal(0))
			})

			It("ignores the routing domains and alias suffixes with a warning that names the error when the recursors cannot be read", func() {
				recursorReader.GetReturns(nil, errors.New("no resolv.conf"))

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal(masterDomains(handlerDomains, recordSet, upcheckDomains)))
				Expect(warnings()).To(ConsistOf(And(
					ContainSubstring("Ignoring alias suffixes and systemd_resolved.routing_domains"),
					ContainSubstring("could not work out its recursors (no resolv.conf)"),
					ContainSubstring("disable_recursors: true"),
				)))
			})

			It("applies the routing domains and alias suffixes when the recursors read from resolv.conf do not include the stub", func() {
				recursorReader.GetReturns([]string{"10.0.0.2:53"}, nil)

				domains := domainsFromConfig(cfg, ownSuffixFile, handlerDomains, recordSet, fs, recursorReader, fakeLogger)

				Expect(domains).To(Equal([]string{
					"corp.intranet.local.",
					"bosh.",
					"service.cf.internal.",
					"bosh-dns.",
				}))
				Expect(fakeLogger.WarnCallCount()).To(Equal(0))
			})
		})
	})

	Describe("updateSystemdResolved", func() {
		var (
			cfg                    config.Config
			fs                     *boshsysfakes.FakeFileSystem
			fakeLogger             *loggerfakes.FakeLogger
			systemdResolvedManager *managerfakes.FakeSystemdResolvedManager
			handlersConfiguration  handlers.HandlerConfigs
			recursorReader         *configfakes.FakeRecursorReader
		)

		BeforeEach(func() {
			fs = boshsysfakes.NewFakeFileSystem()
			fakeLogger = &loggerfakes.FakeLogger{}
			systemdResolvedManager = &managerfakes.FakeSystemdResolvedManager{}
			recursorReader = &configfakes.FakeRecursorReader{}
			handlersConfiguration = handlers.HandlerConfigs{{Domain: "corp.intranet.local."}, {Domain: "bosh."}}

			cfg = config.NewDefaultConfig()
			cfg.ConfigureSystemdResolved = true
			cfg.UpcheckDomains = upcheckDomains
		})

		It("sends the handler, record set, and upcheck parent domains", func() {
			err := updateSystemdResolved(systemdResolvedManager, cfg, "", handlersConfiguration, newRecordSet(aliases.NewConfig()), fs, recursorReader, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
			Expect(systemdResolvedManager.UpdateDomainsCallCount()).To(Equal(1))
			Expect(systemdResolvedManager.UpdateDomainsArgsForCall(0)).To(Equal([]string{
				"corp.intranet.local.",
				"bosh.",
				"bosh.",
				"instance0.my-group.bosh.",
				"bosh-dns.",
				"bosh.",
			}))
			Expect(fakeLogger.WarnCallCount()).To(Equal(0))
		})

		It("returns the error from systemd-resolved", func() {
			systemdResolvedManager.UpdateDomainsReturns(errors.New("resolvectl failed"))

			err := updateSystemdResolved(systemdResolvedManager, cfg, "", handlersConfiguration, newRecordSet(aliases.NewConfig()), fs, recursorReader, fakeLogger)

			Expect(err).To(MatchError("resolvectl failed"))
		})

		It("does nothing when configure_systemd_resolved is false", func() {
			cfg.ConfigureSystemdResolved = false

			err := updateSystemdResolved(systemdResolvedManager, cfg, "", handlersConfiguration, newRecordSet(aliases.NewConfig()), fs, recursorReader, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
			Expect(systemdResolvedManager.UpdateDomainsCallCount()).To(Equal(0))
		})

		It("warns before it sends more distinct domains than systemd-resolved accepts", func() {
			recordsJSON = largeDeploymentRecordsJSON(300, 1)

			err := updateSystemdResolved(systemdResolvedManager, cfg, "", nil, newRecordSet(aliases.NewConfig()), fs, recursorReader, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
			Expect(systemdResolvedManager.UpdateDomainsArgsForCall(0)).To(HaveLen(303))
			Expect(fakeLogger.WarnCallCount()).To(Equal(1))
			_, message, args := fakeLogger.WarnArgsForCall(0)
			Expect(fmt.Sprintf(message, args...)).To(HavePrefix("Sending 302 domains to systemd-resolved, which accepts at most 256 on one link"))
		})

		It("does not warn at the limit or for repeated domains", func() {
			recordsJSON = largeDeploymentRecordsJSON(253, 1)

			err := updateSystemdResolved(systemdResolvedManager, cfg, "", handlersConfiguration, newRecordSet(aliases.NewConfig()), fs, recursorReader, fakeLogger)

			Expect(err).ToNot(HaveOccurred())
			Expect(systemdResolvedManager.UpdateDomainsArgsForCall(0)).To(HaveLen(258))
			Expect(fakeLogger.WarnCallCount()).To(Equal(0))
		})
	})

	Describe("forwardsToStubResolver", func() {
		DescribeTable("reports whether bosh-dns can forward to the systemd-resolved stub resolver",
			func(disableRecursors bool, recursors []string, discovered []string, excludedRecursors []string, expected bool) {
				cfg := config.NewDefaultConfig()
				cfg.ConfigureSystemdResolved = true
				cfg.DisableRecursors = disableRecursors
				cfg.Recursors = recursors
				cfg.ExcludedRecursors = excludedRecursors
				recursorReader := &configfakes.FakeRecursorReader{}
				recursorReader.GetReturns(discovered, nil)

				forwards, err := forwardsToStubResolver(cfg, recursorReader)

				Expect(err).ToNot(HaveOccurred())
				Expect(forwards).To(Equal(expected))
				Expect(cfg.Recursors).To(Equal(recursors))
				if disableRecursors || len(recursors) > 0 {
					Expect(recursorReader.GetCallCount()).To(Equal(0))
				}
			},
			Entry("recursors disabled", true, nil, []string{"127.0.0.53:53"}, nil, false),
			Entry("the stub read from resolv.conf", false, nil, []string{"127.0.0.53:53"}, nil, true),
			Entry("the proxy stub read from resolv.conf", false, nil, []string{"127.0.0.54:53"}, nil, true),
			Entry("an upstream read from resolv.conf", false, nil, []string{"10.0.0.2:53"}, nil, false),
			Entry("the stub after an upstream read from resolv.conf", false, nil, []string{"10.0.0.2:53", "127.0.0.53:53"}, nil, true),
			Entry("nothing read from resolv.conf", false, nil, []string{}, nil, false),
			Entry("the proxy stub read from resolv.conf and excluded", false, nil, []string{"127.0.0.54:53"}, []string{"127.0.0.54:53"}, false),
			Entry("the proxy stub read from resolv.conf with the other stub excluded", false, nil, []string{"127.0.0.54:53"}, []string{"127.0.0.53:53"}, true),
			Entry("explicit recursors", false, []string{"10.0.0.2:53"}, []string{"127.0.0.53:53"}, nil, false),
			Entry("the stub as a recursor", false, []string{"127.0.0.53"}, nil, nil, true),
			Entry("the stub with its port", false, []string{"127.0.0.53:53"}, nil, nil, true),
			Entry("the proxy stub", false, []string{"127.0.0.54:53"}, nil, nil, true),
			Entry("the stub after another recursor", false, []string{"10.0.0.2:53", "127.0.0.53:53"}, nil, nil, true),
			Entry("the stub address on another port", false, []string{"127.0.0.53:5353"}, nil, nil, false),
			Entry("the stub as an IPv4-mapped address", false, []string{"[::ffff:127.0.0.53]:53"}, nil, nil, true),
			Entry("an explicit stub excluded", false, []string{"127.0.0.53"}, nil, []string{"127.0.0.53:53"}, false),
			Entry("the other stub excluded", false, []string{"127.0.0.53:53"}, nil, []string{"127.0.0.54:53"}, true),
			Entry("an explicit stub excluded as an IPv4-mapped address with a port, which the server does not match", false, []string{"127.0.0.53"}, nil, []string{"[::ffff:127.0.0.53]:53"}, true),
			Entry("an explicit stub excluded as an IPv4-mapped address without a port, which the server reads as 127.0.0.53:53", false, []string{"127.0.0.53"}, nil, []string{"::ffff:127.0.0.53"}, false),
			Entry("the stub read from resolv.conf excluded as an IPv4-mapped address with a port", false, nil, []string{"127.0.0.53:53"}, []string{"[::ffff:127.0.0.53]:53"}, true),
		)

		It("reports true with the error when the recursors cannot be read", func() {
			cfg := config.NewDefaultConfig()
			cfg.ConfigureSystemdResolved = true
			recursorReader := &configfakes.FakeRecursorReader{}
			recursorReader.GetReturns(nil, errors.New("no resolv.conf"))

			forwards, err := forwardsToStubResolver(cfg, recursorReader)

			Expect(err).To(MatchError("no resolv.conf"))
			Expect(forwards).To(BeTrue())
		})

		It("reports true with the error when a recursor is not an address", func() {
			cfg := config.NewDefaultConfig()
			cfg.ConfigureSystemdResolved = true
			cfg.Recursors = []string{"not-an-address"}
			recursorReader := &configfakes.FakeRecursorReader{}

			forwards, err := forwardsToStubResolver(cfg, recursorReader)

			Expect(err).To(MatchError(ContainSubstring("not-an-address")))
			Expect(forwards).To(BeTrue())
		})

		It("reports false when configure_systemd_resolved is false", func() {
			cfg := config.NewDefaultConfig()
			recursorReader := &configfakes.FakeRecursorReader{}

			forwards, err := forwardsToStubResolver(cfg, recursorReader)

			Expect(err).ToNot(HaveOccurred())
			Expect(forwards).To(BeFalse())
			Expect(recursorReader.GetCallCount()).To(Equal(0))
		})
	})

	Describe("serverRecursorReader", func() {
		var (
			fs         *boshsysfakes.FakeFileSystem
			dnsManager *managerfakes.FakeDNSManager
			reader     serverRecursorReader
		)

		BeforeEach(func() {
			fs = boshsysfakes.NewFakeFileSystem()
			dnsManager = &managerfakes.FakeDNSManager{}
			reader = serverRecursorReader{
				address:            "169.254.0.2",
				addressesFilesGlob: "/var/vcap/jobs/*/dns/addresses.json",
				fs:                 fs,
				dnsManager:         dnsManager,
			}

			Expect(fs.WriteFileString("/var/vcap/jobs/a/dns/addresses.json", `[{"address": "169.254.0.3", "port": 53}]`)).To(Succeed())
			fs.SetGlob("/var/vcap/jobs/*/dns/addresses.json", []string{"/var/vcap/jobs/a/dns/addresses.json"})
		})

		It("reads the nameservers without bosh-dns's own addresses and never sets bosh-dns as the primary", func() {
			dnsManager.ReadReturns([]string{"169.254.0.2", "169.254.0.3", "127.0.0.1", "127.0.0.53"}, nil)

			Expect(reader.Get()).To(Equal([]string{"127.0.0.53:53"}))
			Expect(dnsManager.ReadCallCount()).To(Equal(1))
			Expect(dnsManager.SetPrimaryCallCount()).To(Equal(0))
		})

		It("returns the error when an addresses file cannot be read", func() {
			fs.RegisterReadFileError("/var/vcap/jobs/a/dns/addresses.json", errors.New("permission denied"))

			_, err := reader.Get()

			Expect(err).To(MatchError(ContainSubstring("permission denied")))
			Expect(dnsManager.ReadCallCount()).To(Equal(0))
		})

		It("returns the error when resolv.conf cannot be read", func() {
			dnsManager.ReadReturns(nil, errors.New("no resolv.conf"))

			_, err := reader.Get()

			Expect(err).To(MatchError("no resolv.conf"))
		})
	})

	Describe("ownAliasSuffixFile", func() {
		It("returns the alias suffix file of the job that the config file belongs to", func() {
			Expect(ownAliasSuffixFile("/var/vcap/jobs/bosh-dns/config/config.json")).To(Equal("/var/vcap/jobs/bosh-dns/dns/alias_suffixes.json"))
		})
	})
})

// largeDeploymentRecordsJSON returns a records file with instances spread
// across groups, each group with a uuid placeholder alias, the way the
// director writes them for a large deployment.
func largeDeploymentRecordsJSON(instances int, groups int) string {
	recordInfos := ""
	for i := 0; i < instances; i++ {
		if i > 0 {
			recordInfos += ","
		}
		group := i % groups
		id := fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i)
		recordInfos += fmt.Sprintf(
			`["%s", "%d", "group-%d", ["%d"], "z1", "1", "default", "1", "cf", "10.0.%d.%d", "bosh."]`,
			id, i, group, group, i/250, i%250,
		)
	}

	recordAliases := ""
	for group := 0; group < groups; group++ {
		if group > 0 {
			recordAliases += ","
		}
		recordAliases += fmt.Sprintf(
			`"_.group-%d.bosh": [{"group_id": "%d", "root_domain": "bosh", "placeholder_type": "uuid"}]`,
			group, group,
		)
	}

	return fmt.Sprintf(`{
		"record_keys": ["id", "num_id", "instance_group", "group_ids", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
		"record_infos": [%s],
		"aliases": {%s}
	}`, recordInfos, recordAliases)
}

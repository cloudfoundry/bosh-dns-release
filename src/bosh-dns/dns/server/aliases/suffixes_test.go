package aliases_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/aliases/aliasesfakes"
)

// domainCorpus is shared with the bosh-dns job spec, so that the template
// and the updater accept and reject the same names.
type domainCorpus struct {
	Domains []struct {
		Domain string `json:"domain"`
		Error  string `json:"error"`
	} `json:"domains"`
	AliasSuffixes []struct {
		Alias  string `json:"alias"`
		Suffix string `json:"suffix"`
		Error  string `json:"error"`
	} `json:"alias_suffixes"`
}

func loadDomainCorpus() domainCorpus {
	contents, err := os.ReadFile("testdata/domain_corpus.json")
	Expect(err).ToNot(HaveOccurred())

	var corpus domainCorpus
	Expect(json.Unmarshal(contents, &corpus)).To(Succeed())
	Expect(corpus.Domains).ToNot(BeEmpty())
	Expect(corpus.AliasSuffixes).ToNot(BeEmpty())

	return corpus
}

var _ = Describe("CanonicalDomain", func() {
	DescribeTable("lowercases and qualifies a valid domain",
		func(rawDomain string, expected string) {
			Expect(aliases.CanonicalDomain(rawDomain)).To(Equal(expected))
		},
		Entry("single label", "bosh", "bosh."),
		Entry("trailing dot", "service.cf.internal.", "service.cf.internal."),
		Entry("mixed case", "Service.CF.Internal", "service.cf.internal."),
	)

	It("accepts and rejects every domain in the shared corpus", func() {
		for _, entry := range loadDomainCorpus().Domains {
			_, err := aliases.CanonicalDomain(entry.Domain)
			if entry.Error == "" {
				Expect(err).ToNot(HaveOccurred(), "domain %q", entry.Domain)
			} else {
				Expect(err).To(MatchError(entry.Error), "domain %q", entry.Domain)
			}
		}
	})
})

var _ = Describe("Suffixes", func() {
	Describe("NewSuffixesFromMap", func() {
		It("stores the suffix of an underscore alias apart from a plain alias on the same host", func() {
			suffixes := MustNewSuffixesFromMap(map[string]string{
				"bbs.service.cf.internal": "service.cf.internal",
				"_.under.foo.bar":         "bar",
			})

			defined, undefined := suffixes.DefinedIn(MustNewConfigFromMap(map[string][]string{
				"bbs.service.cf.internal": {"domain1"},
				"under.foo.bar":           {"domain2"},
			}))

			Expect(defined).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
			Expect(undefined).To(Equal([]string{"_.under.foo.bar."}))
		})

		It("reports an empty map as empty", func() {
			Expect(MustNewSuffixesFromMap(map[string]string{}).IsEmpty()).To(BeTrue())
			Expect(aliases.NewSuffixes().IsEmpty()).To(BeTrue())
		})

		It("accepts and rejects every alias suffix in the shared corpus and keeps the valid entries", func() {
			for _, entry := range loadDomainCorpus().AliasSuffixes {
				suffixes, errs := aliases.NewSuffixesFromMap(map[string]string{
					entry.Alias:     entry.Suffix,
					"keep.internal": "internal",
				})

				if entry.Error == "" {
					Expect(errs).To(BeEmpty(), "alias %q with suffix %q", entry.Alias, entry.Suffix)
				} else {
					Expect(errs).To(ConsistOf(MatchError(And(
						HavePrefix(fmt.Sprintf("alias %q: ", entry.Alias)),
						ContainSubstring(entry.Error),
					))), "alias %q with suffix %q", entry.Alias, entry.Suffix)
					Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"keep.internal": "internal"})))
				}
			}
		})

		DescribeTable("reports an invalid entry by alias",
			func(alias string, suffix string, message string) {
				_, errs := aliases.NewSuffixesFromMap(map[string]string{alias: suffix})

				Expect(errs).To(ConsistOf(MatchError(message)))
			},
			Entry("invalid suffix", "bbs.service.cf.internal", "~service.cf.internal", `alias "bbs.service.cf.internal": suffix "~service.cf.internal": write it without the ~ prefix`),
			Entry("invalid alias", `x\.svc.internal`, "svc.internal", `alias "x\\.svc.internal": invalid alias: not a valid domain name`),
			Entry("unrelated suffix", "bbs.service.cf.internal", "other.internal", `alias "bbs.service.cf.internal": suffix "other.internal." is not the alias or a parent of it`),
		)

		It("keeps the first valid entry in sorted key order when keys differ only in case", func() {
			suffixes, errs := aliases.NewSuffixesFromMap(map[string]string{
				"bbs.service.cf.internal": "cf.internal",
				"BBS.Service.CF.Internal": "service.cf.internal",
			})

			Expect(errs).To(ConsistOf(MatchError(`alias "bbs.service.cf.internal": the alias already has the suffix "service.cf.internal."`)))
			Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		})

		It("accepts keys that differ only in case when they give the same suffix", func() {
			suffixes, errs := aliases.NewSuffixesFromMap(map[string]string{
				"bbs.service.cf.internal": "service.cf.internal",
				"BBS.Service.CF.Internal": "Service.CF.Internal.",
			})

			Expect(errs).To(BeEmpty())
			Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		})

		It("lets a later key win when the key that sorts first is invalid", func() {
			suffixes, errs := aliases.NewSuffixesFromMap(map[string]string{
				"BBS.Service.CF.Internal": "other.internal",
				"bbs.service.cf.internal": "service.cf.internal",
			})

			Expect(errs).To(ConsistOf(MatchError(`alias "BBS.Service.CF.Internal": suffix "other.internal." is not the alias or a parent of it`)))
			Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		})
	})

	Describe("DefinedIn", func() {
		It("splits the suffixes into those for defined aliases and the names of undefined aliases", func() {
			defined, undefined := MustNewSuffixesFromMap(map[string]string{
				"a.service.internal":   "service.internal",
				"_.under.internal":     "internal",
				"missing.internal":     "internal",
				"_.missing.internal":   "internal",
				"under.other.internal": "other.internal",
			}).DefinedIn(MustNewConfigFromMap(map[string][]string{
				"a.service.internal": {"domain1"},
				"_.under.internal":   {"_.domain2"},
				"_.other.internal":   {"_.domain3"},
			}))

			Expect(defined).To(Equal(MustNewSuffixesFromMap(map[string]string{
				"a.service.internal": "service.internal",
				"_.under.internal":   "internal",
			})))
			Expect(undefined).To(Equal([]string{"_.missing.internal.", "missing.internal.", "under.other.internal."}))
		})
	})
})

var _ = Describe("FSSuffixesLoader", func() {
	var (
		fs     *boshsysfakes.FakeFileSystem
		loader aliases.NamedSuffixesLoader
	)

	BeforeEach(func() {
		fs = boshsysfakes.NewFakeFileSystem()
		loader = aliases.NewFSSuffixesLoader(fs)
	})

	It("loads the suffixes in a file", func() {
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{"bbs.service.cf.internal": "service.cf.internal", "_.under.internal": "internal"}`)).To(Succeed())

		suffixes, errs, err := loader.Load("/jobs/a/dns/alias_suffixes.json")

		Expect(err).ToNot(HaveOccurred())
		Expect(errs).To(BeEmpty())
		Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{
			"bbs.service.cf.internal": "service.cf.internal",
			"_.under.internal":        "internal",
		})))
	})

	It("loads an empty object or null as no suffixes", func() {
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/b/dns/alias_suffixes.json", `null`)).To(Succeed())

		for _, file := range []string{"/jobs/a/dns/alias_suffixes.json", "/jobs/b/dns/alias_suffixes.json"} {
			suffixes, errs, err := loader.Load(file)
			Expect(err).ToNot(HaveOccurred())
			Expect(errs).To(BeEmpty())
			Expect(suffixes.IsEmpty()).To(BeTrue())
		}
	})

	DescribeTable("fails on a file that cannot be used",
		func(contents string, message string) {
			Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", contents)).To(Succeed())

			_, _, err := loader.Load("/jobs/a/dns/alias_suffixes.json")

			Expect(err).To(MatchError(HavePrefix(message)))
		},
		Entry("truncated", `{"bbs.service.cf.internal": `, "is not a JSON object: unexpected end of JSON input"),
		Entry("an array", `["service.cf.internal"]`, "is not a JSON object: json: cannot unmarshal array"),
	)

	It("fails on a file that cannot be read", func() {
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{}`)).To(Succeed())
		fs.RegisterReadFileError("/jobs/a/dns/alias_suffixes.json", errors.New("permission denied"))

		_, _, err := loader.Load("/jobs/a/dns/alias_suffixes.json")

		Expect(err).To(MatchError("could not be read: permission denied"))
	})

	It("returns an error in sorted order for each entry it skips and keeps the rest", func() {
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{
			"object.internal": {"suffix": "internal"},
			"number.internal": 3,
			"null.internal": null,
			"bad.internal": "~internal",
			"keep.internal": "internal"
		}`)).To(Succeed())

		suffixes, errs, err := loader.Load("/jobs/a/dns/alias_suffixes.json")

		Expect(err).ToNot(HaveOccurred())
		Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"keep.internal": "internal"})))
		Expect(errs).To(HaveLen(4))
		Expect(errs[0]).To(MatchError(`alias "null.internal": the suffix is not a string`))
		Expect(errs[1]).To(MatchError(`alias "number.internal": the suffix is not a string`))
		Expect(errs[2]).To(MatchError(`alias "object.internal": the suffix is not a string`))
		Expect(errs[3]).To(MatchError(`alias "bad.internal": suffix "~internal": write it without the ~ prefix`))
	})
})

var _ = Describe("SuffixesFromGlob", func() {
	const (
		ownFile = "/jobs/bosh-dns/dns/alias_suffixes.json"
		glob    = "/jobs/*/dns/alias_suffixes.json"
	)

	var (
		fs         *boshsysfakes.FakeFileSystem
		fakeLogger *loggerfakes.FakeLogger
	)

	BeforeEach(func() {
		fs = boshsysfakes.NewFakeFileSystem()
		fakeLogger = &loggerfakes.FakeLogger{}
	})

	logged := func(callCount func() int, argsForCall func(int) (string, string, []interface{})) []string {
		var messages []string
		for i := 0; i < callCount(); i++ {
			tag, message, args := argsForCall(i)
			Expect(tag).To(Equal("AliasSuffixes"))
			messages = append(messages, fmt.Sprintf(message, args...))
		}
		return messages
	}
	warnings := func() []string { return logged(fakeLogger.WarnCallCount, fakeLogger.WarnArgsForCall) }
	infos := func() []string { return logged(fakeLogger.InfoCallCount, fakeLogger.InfoArgsForCall) }

	load := func() aliases.Suffixes {
		return aliases.SuffixesFromGlob(fs, aliases.NewFSSuffixesLoader(fs), ownFile, glob, fakeLogger)
	}

	It("loads every file that matches the glob and keeps the first suffix for each alias", func() {
		fs.SetGlob(glob, []string{"/jobs/a/dns/alias_suffixes.json", "/jobs/b/dns/alias_suffixes.json"})
		Expect(fs.WriteFileString(ownFile, `{}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{"bbs.service.cf.internal": "service.cf.internal"}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/b/dns/alias_suffixes.json", `{"bbs.service.cf.internal": "cf.internal", "_.under.internal": "internal"}`)).To(Succeed())

		Expect(load()).To(Equal(MustNewSuffixesFromMap(map[string]string{
			"bbs.service.cf.internal": "service.cf.internal",
			"_.under.internal":        "internal",
		})))
		Expect(warnings()).To(BeEmpty())
		Expect(infos()).To(ConsistOf(
			"Ignoring the suffix cf.internal. for alias bbs.service.cf.internal. from /jobs/b/dns/alias_suffixes.json because /jobs/a/dns/alias_suffixes.json gives it the suffix service.cf.internal.",
		))
	})

	It("lets the bosh-dns job's own file win over files that sort before it, and reads it once", func() {
		fs.SetGlob(glob, []string{"/jobs/api/dns/alias_suffixes.json", ownFile, "/jobs/zed/dns/alias_suffixes.json"})
		Expect(fs.WriteFileString("/jobs/api/dns/alias_suffixes.json", `{"bbs.service.cf.internal": "bbs.service.cf.internal", "_.under.internal": "under.internal"}`)).To(Succeed())
		Expect(fs.WriteFileString(ownFile, `{"bbs.service.cf.internal": "service.cf.internal", "_.under.internal": "internal"}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/zed/dns/alias_suffixes.json", `{"other.internal": "internal"}`)).To(Succeed())
		loader := &aliasesfakes.FakeNamedSuffixesLoader{}
		loader.LoadCalls(aliases.NewFSSuffixesLoader(fs).Load)

		suffixes := aliases.SuffixesFromGlob(fs, loader, ownFile, glob, fakeLogger)

		Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{
			"bbs.service.cf.internal": "service.cf.internal",
			"_.under.internal":        "internal",
			"other.internal":          "internal",
		})))
		Expect(loader.LoadCallCount()).To(Equal(3))
		Expect(loader.LoadArgsForCall(0)).To(Equal(ownFile))
		Expect(loader.LoadArgsForCall(1)).To(Equal("/jobs/api/dns/alias_suffixes.json"))
		Expect(loader.LoadArgsForCall(2)).To(Equal("/jobs/zed/dns/alias_suffixes.json"))
		Expect(infos()).To(Equal([]string{
			"Ignoring the suffix bbs.service.cf.internal. for alias bbs.service.cf.internal. from /jobs/api/dns/alias_suffixes.json because /jobs/bosh-dns/dns/alias_suffixes.json gives it the suffix service.cf.internal.",
			"Ignoring the suffix under.internal. for alias _.under.internal. from /jobs/api/dns/alias_suffixes.json because /jobs/bosh-dns/dns/alias_suffixes.json gives it the suffix internal.",
		}))
	})

	It("reads the bosh-dns job's own file when the glob does not match it", func() {
		Expect(fs.WriteFileString(ownFile, `{"bbs.service.cf.internal": "service.cf.internal"}`)).To(Succeed())

		Expect(load()).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		Expect(warnings()).To(BeEmpty())
	})

	It("does not log a file that gives an alias the same suffix", func() {
		fs.SetGlob(glob, []string{"/jobs/a/dns/alias_suffixes.json"})
		Expect(fs.WriteFileString(ownFile, `{"bbs.service.cf.internal": "service.cf.internal"}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/a/dns/alias_suffixes.json", `{"BBS.service.cf.internal": "service.cf.internal."}`)).To(Succeed())

		Expect(load()).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		Expect(infos()).To(BeEmpty())
		Expect(warnings()).To(BeEmpty())
	})

	It("returns no suffixes when no file exists or matches", func() {
		Expect(aliases.SuffixesFromGlob(fs, aliases.NewFSSuffixesLoader(fs), "", glob, fakeLogger).IsEmpty()).To(BeTrue())
		Expect(warnings()).To(BeEmpty())
	})

	It("logs and skips a glob that fails and still reads the bosh-dns job's own file", func() {
		fs.GlobErrs = map[string]error{"/jobs/[/alias_suffixes.json": errors.New("bad pattern")}
		Expect(fs.WriteFileString(ownFile, `{"bbs.service.cf.internal": "service.cf.internal"}`)).To(Succeed())

		suffixes := aliases.SuffixesFromGlob(fs, aliases.NewFSSuffixesLoader(fs), ownFile, "/jobs/[/alias_suffixes.json", fakeLogger)

		Expect(suffixes).To(Equal(MustNewSuffixesFromMap(map[string]string{"bbs.service.cf.internal": "service.cf.internal"})))
		Expect(warnings()).To(ConsistOf("Ignoring alias suffix files matching /jobs/[/alias_suffixes.json because the glob failed: bad pattern"))
	})

	It("logs and skips a file or entry that cannot be used and loads the rest", func() {
		fs.SetGlob(glob, []string{"/jobs/50%d/dns/alias_suffixes.json", "/jobs/b/dns/alias_suffixes.json", "/jobs/c/dns/alias_suffixes.json"})
		Expect(fs.WriteFileString(ownFile, `{}`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/50%d/dns/alias_suffixes.json", `{"lost.internal": "internal"}`)).To(Succeed())
		fs.RegisterReadFileError("/jobs/50%d/dns/alias_suffixes.json", errors.New("permission denied"))
		Expect(fs.WriteFileString("/jobs/b/dns/alias_suffixes.json", `["service.cf.internal"]`)).To(Succeed())
		Expect(fs.WriteFileString("/jobs/c/dns/alias_suffixes.json", `{"50%s.internal": "~internal", "keep.internal": "internal"}`)).To(Succeed())

		Expect(load()).To(Equal(MustNewSuffixesFromMap(map[string]string{"keep.internal": "internal"})))
		Expect(warnings()).To(Equal([]string{
			"Ignoring alias suffix file /jobs/50%d/dns/alias_suffixes.json: could not be read: permission denied",
			"Ignoring alias suffix file /jobs/b/dns/alias_suffixes.json: is not a JSON object: json: cannot unmarshal array into Go value of type map[string]interface {}",
			`Ignoring an entry in alias suffix file /jobs/c/dns/alias_suffixes.json: alias "50%s.internal": suffix "~internal": write it without the ~ prefix`,
		}))
	})
})

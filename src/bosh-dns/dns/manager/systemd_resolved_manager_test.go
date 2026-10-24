package manager_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"bosh-dns/dns/manager"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemdResolvedManager", func() {
	var (
		fakeCmdRunner *fakes.FakeCmdRunner
		logger        *loggerfakes.FakeLogger
		sleeps        []time.Duration
		mgr           interface{ UpdateDomains([]string) error }
	)

	setCmd := func(args ...string) []string {
		return append([]string{"resolvectl", "domain", "bosh-dns"}, args...)
	}
	readCmd := setCmd()

	// readback registers what `resolvectl domain bosh-dns` prints for every call.
	readback := func(tokens ...string) {
		fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{
			Stdout: "Link 5 (bosh-dns): " + strings.Join(tokens, " ") + "\n",
			Sticky: true,
		})
	}

	warnings := func() []string {
		var messages []string
		for i := 0; i < logger.WarnCallCount(); i++ {
			_, msg, args := logger.WarnArgsForCall(i)
			messages = append(messages, fmt.Sprintf(msg, args...))
		}
		return messages
	}

	BeforeEach(func() {
		fakeCmdRunner = fakes.NewFakeCmdRunner()
		logger = &loggerfakes.FakeLogger{}
		sleeps = nil
		mgr = manager.NewSystemdResolvedManagerWithSleeper(fakeCmdRunner, logger, func(d time.Duration) {
			sleeps = append(sleeps, d)
		})
	})

	Describe("UpdateDomains", func() {
		It("sets only the sorted parents, each as a routing-only domain, and reads the link back", func() {
			readback("~bosh", "~bosh-dns", "~other.example")

			err := mgr.UpdateDomains([]string{"foo.bosh.", "other.example", "bosh-dns.", "x.y.bosh", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh-dns.", "~bosh.", "~other.example."),
				readCmd,
			}))
		})

		It("sorts the canonical names", func() {
			readback("~a.example", "~b.example", "~c.example")

			err := mgr.UpdateDomains([]string{"c.example.", "a.example.", "b.example."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~a.example.", "~b.example.", "~c.example.")))
		})

		It("dedupes escaped, differently cased, and unqualified spellings of one name", func() {
			readback("~bosh")

			err := mgr.UpdateDomains([]string{`\098osh.`, "BOSH.", "bosh", `\066OSH`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
		})

		It("does not double-prefix a name that already starts with a tilde", func() {
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"~bosh.", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
		})

		It("accepts 257 inputs that dedupe to exactly 256 names and passes every one", func() {
			var domains, expected, tokens []string
			for i := 0; i < manager.MaxSystemdResolvedLinkDomains; i++ {
				domains = append(domains, fmt.Sprintf("Domain-%03d.example.", i))
				expected = append(expected, fmt.Sprintf("~domain-%03d.example.", i))
				tokens = append(tokens, fmt.Sprintf("~domain-%03d.example", i))
			}
			domains = append(domains, "domain-000.example")
			Expect(domains).To(HaveLen(257))
			readback(tokens...)

			err := mgr.UpdateDomains(domains)

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(expected...)))
		})

		It("keeps a name with an escaped dot next to the name it resembles", func() {
			readback(`~bar.bosh`, `~foo\.bar.bosh`)

			err := mgr.UpdateDomains([]string{`foo\.bar.bosh.`, "bar.bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bar.bosh.", `~foo\.bar.bosh.`)))
		})

		It("prunes a subdomain of an escaped-dot name but not the name itself", func() {
			readback(`~foo\.bar.bosh`)

			err := mgr.UpdateDomains([]string{`a.foo\.bar.bosh.`, `foo\.bar.bosh.`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~foo\.bar.bosh.`)))
		})

		It("compares whole labels when pruning", func() {
			readback("~bosh", "~notbosh")

			err := mgr.UpdateDomains([]string{"notbosh.", "bosh.", "abc.bosh.", "c.bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.", "~notbosh.")))
		})

		It("sets the root as ~. with a warning and does not let it prune other names", func() {
			readback("~.", "~bosh")

			err := mgr.UpdateDomains([]string{"bosh.", ".", "foo.bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~.", "~bosh.")))
			Expect(warnings()).To(ContainElement(ContainSubstring("root")))
		})

		It("keeps names that differ only by whitespace, non-ASCII bytes, or a look-alike code point such as the Kelvin sign distinct", func() {
			readback("~bosh", `~\032bosh`, `~b\195\182sh`, "~k.example", `~\226\132\170.example`)

			err := mgr.UpdateDomains([]string{"bosh.", " bosh.", "bösh.", "k.example.", "K.example."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(
				`~\032bosh.`, `~\226\132\170.example.`, `~b\195\182sh.`, "~bosh.", "~k.example.",
			)))
		})

		It("does not fold the Kelvin sign U+212A into ASCII k", func() {
			readback("~k.example", `~\226\132\170.example`)

			err := mgr.UpdateDomains([]string{"K.example.", "K.example."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~\226\132\170.example.`, "~k.example.")))
		})

		It("writes escapes that systemd accepts and keeps the plain spelling apart", func() {
			readback(`~\032bosh`, "~bosh")

			err := mgr.UpdateDomains([]string{" bosh.", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~\032bosh.`, "~bosh.")))
		})

		It("writes a three-digit escape for an escaped at sign", func() {
			readback(`~a\064b.example`)

			err := mgr.UpdateDomains([]string{"a@b.example."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~a\064b.example.`)))
		})

		DescribeTable("skips a name with an invalid decimal escape and prunes nothing",
			func(invalid string) {
				readback("~bosh")

				err := mgr.UpdateDomains([]string{invalid, "bosh.", "child.bosh."})

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
				Expect(warnings()).To(ContainElement(ContainSubstring("Skipping invalid domain")))
			},
			Entry("256", `\256osh.`),
			Entry("354", `\354osh.`),
			Entry("999", `\999osh.`),
			Entry("too few digits", `\25`),
			Entry("a trailing backslash", `osh\`),
		)

		It("keeps an escaped backslash followed by digits", func() {
			readback(`~\\354osh`)

			err := mgr.UpdateDomains([]string{`\\354osh.`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~\\354osh.`)))
			Expect(warnings()).To(BeEmpty())
		})

		It("warns about the root whatever the sort order", func() {
			readback(`~\033`, "~.", "~bosh")

			err := mgr.UpdateDomains([]string{"!.", ".", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(warnings()).To(ContainElement(ContainSubstring("root")))
		})

		DescribeTable("clears the link with a single empty argument when nothing is left to set",
			func(domains []string) {
				readback()

				err := mgr.UpdateDomains(domains)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
					readCmd,
					setCmd(""),
					readCmd,
				}))
			},
			Entry("an empty slice", []string{}),
			Entry("a nil slice", nil),
		)

		It("skips an invalid name with a warning that names it and still sets the rest", func() {
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"a..b.", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
			Expect(warnings()).To(ContainElement(ContainSubstring(`"a..b."`)))
		})

		It("skips an empty name instead of treating it as the root", func() {
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"", "~", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
			Expect(warnings()).To(HaveLen(2))
		})

		It("covers 300 instance aliases and 25 service aliases with 27 entries", func() {
			var domains, expected, tokens []string
			for i := 0; i < 300; i++ {
				domains = append(domains, fmt.Sprintf("00000000-0000-0000-0000-%012d.example_service.bosh.", i))
			}
			domains = append(domains, "bosh.", "bosh-dns.")
			expected = append(expected, "~bosh.", "~bosh-dns.")
			tokens = append(tokens, "~bosh", "~bosh-dns")
			for i := 0; i < 25; i++ {
				domains = append(domains, fmt.Sprintf("service-%02d.service.cf.internal.", i))
				expected = append(expected, fmt.Sprintf("~service-%02d.service.cf.internal.", i))
				tokens = append(tokens, fmt.Sprintf("~service-%02d.service.cf.internal", i))
			}
			sort.Strings(expected)
			readback(tokens...)

			err := mgr.UpdateDomains(domains)

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(expected...)))
			Expect(fakeCmdRunner.RunCommands[1]).To(HaveLen(3 + 27))
		})

		It("builds the same argv whatever the order of the input", func() {
			readback("~a.example", "~bosh", "~z.example")

			err := mgr.UpdateDomains([]string{"z.example.", "x.bosh.", "bosh.", "a.example."})
			Expect(err).NotTo(HaveOccurred())

			err = mgr.UpdateDomains([]string{"a.example.", "bosh.", "x.bosh.", "z.example."})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCmdRunner.RunCommands).To(HaveLen(6))
			Expect(fakeCmdRunner.RunCommands[4]).To(Equal(fakeCmdRunner.RunCommands[1]))
		})

		It("returns an error naming the count and the limit, without running resolvectl, when too many zones remain", func() {
			var domains []string
			for i := 0; i < manager.MaxSystemdResolvedLinkDomains+10; i++ {
				domains = append(domains, fmt.Sprintf("domain-%d.example", i))
			}

			err := mgr.UpdateDomains(domains)

			Expect(err).To(MatchError(ContainSubstring("266 domains")))
			Expect(err).To(MatchError(ContainSubstring("at most 256")))
			Expect(fakeCmdRunner.RunCommands).To(BeEmpty())
			Expect(logger.ErrorCallCount()).To(Equal(1))
		})

		It("returns the error for exactly 257 unrelated zones", func() {
			var domains []string
			for i := 0; i < manager.MaxSystemdResolvedLinkDomains+1; i++ {
				domains = append(domains, fmt.Sprintf("domain-%d.example", i))
			}

			err := mgr.UpdateDomains(domains)

			Expect(err).To(MatchError(ContainSubstring("257 domains")))
			Expect(fakeCmdRunner.RunCommands).To(BeEmpty())
		})
	})

	Describe("names", func() {
		It("keeps the last label of a name that ends in an escaped dot after a multibyte character", func() {
			readback(`~k\195\128\.`)

			err := mgr.UpdateDomains([]string{`kÀ\.`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~k\195\128\..`)))
		})

		It("does not prune a sibling under a name that ends in an escaped dot after a multibyte character", func() {
			readback(`~a.b\195\128\.`, "~z.a")

			err := mgr.UpdateDomains([]string{`a.bÀ\.`, "z.a."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~a.b\195\128\..`, "~z.a.")))
		})

		It("qualifies a name that ends in an escaped backslash after a multibyte character", func() {
			readback(`~k\195\182\\`)

			err := mgr.UpdateDomains([]string{`kö\\`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~k\195\182\\.`)))
			Expect(warnings()).To(BeEmpty())
		})

		It("keeps a multibyte character that follows an escaped backslash", func() {
			readback(`~a\\\195\128`)

			err := mgr.UpdateDomains([]string{`a\\À.`})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd(`~a\\\195\128.`)))
			Expect(warnings()).To(BeEmpty())
		})

		It("skips a name that is too long once escaped for systemd and still sets the rest", func() {
			readback("~bosh")

			err := mgr.UpdateDomains([]string{strings.Repeat("*", 63) + ".bosh.", "bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~bosh.")))
			Expect(warnings()).To(ContainElement(ContainSubstring("257 characters long when escaped for systemd-resolved")))
		})

		It("sets a name that is exactly as long as systemd allows once escaped", func() {
			readback("~" + strings.Repeat(`\042`, 62) + ".bosh")

			err := mgr.UpdateDomains([]string{strings.Repeat("*", 62) + ".bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands[1]).To(Equal(setCmd("~" + strings.Repeat("*", 62) + ".bosh.")))
			Expect(warnings()).To(BeEmpty())
		})
	})

	Describe("plain domains already on the link", func() {
		clearCmd := setCmd("")

		It("clears the link before setting it when it holds plain domains", func() {
			fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{Stdout: "Link 5 (bosh-dns): bosh bosh-dns\n"})
			readback("~bosh", "~bosh-dns")

			err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				clearCmd,
				setCmd("~bosh-dns.", "~bosh."),
				readCmd,
			}))
		})

		It("does not clear a link that holds only routing-only domains", func() {
			readback("~bosh", "~bosh-dns")

			err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh-dns.", "~bosh."),
				readCmd,
			}))
		})

		It("does not clear an empty link", func() {
			fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{Stdout: "Link 5 (bosh-dns):\n"})
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				readCmd,
			}))
		})

		It("sets the domains with a warning when the link cannot be read first", func() {
			fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{Error: errors.New("read boom")})
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				readCmd,
			}))
			Expect(warnings()).To(ContainElement(ContainSubstring("read boom")))
			Expect(sleeps).To(BeEmpty())
		})
	})

	Describe("resolvectl failures", func() {
		setKey := strings.Join(setCmd("~bosh."), " ")
		clearKey := strings.Join(setCmd(""), " ")

		DescribeTable("clears the link and sets it again when resolvectl reports an argument list that is too long",
			func(result fakes.FakeCmdResult) {
				fakeCmdRunner.AddCmdResult(setKey, result)
				readback("~bosh")

				err := mgr.UpdateDomains([]string{"bosh."})

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
					readCmd,
					setCmd("~bosh."),
					setCmd(""),
					setCmd("~bosh."),
					readCmd,
				}))
				Expect(sleeps).To(BeEmpty())
			},
			Entry("in the error", fakes.FakeCmdResult{Error: errors.New("Failed to set domain: Argument list too long"), ExitStatus: 1}),
			Entry("in stderr", fakes.FakeCmdResult{Error: errors.New("exit status 1"), Stderr: "Failed to set domain: Argument list too long", ExitStatus: 1}),
			Entry("in stdout", fakes.FakeCmdResult{Error: errors.New("exit status 1"), Stdout: "Argument list too long", ExitStatus: 1}),
		)

		It("returns the error when the set still overflows after the link was cleared", func() {
			overflow := fakes.FakeCmdResult{Error: errors.New("Argument list too long")}
			fakeCmdRunner.AddCmdResult(setKey, overflow)
			fakeCmdRunner.AddCmdResult(setKey, overflow)

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("Argument list too long")))
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				setCmd(""),
				setCmd("~bosh."),
			}))
		})

		It("returns an error when the clear fails", func() {
			fakeCmdRunner.AddCmdResult(setKey, fakes.FakeCmdResult{Error: errors.New("Argument list too long")})
			for i := 0; i < 3; i++ {
				fakeCmdRunner.AddCmdResult(clearKey, fakes.FakeCmdResult{Error: errors.New("clear boom")})
			}

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("Clearing domains")))
			Expect(err).To(MatchError(ContainSubstring("clear boom")))
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				setCmd(""),
				setCmd(""),
				setCmd(""),
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second, 2 * time.Second}))
		})

		It("returns the last error when the set after a clear fails three times for another reason", func() {
			fakeCmdRunner.AddCmdResult(setKey, fakes.FakeCmdResult{Error: errors.New("Argument list too long")})
			for i := 1; i <= 3; i++ {
				fakeCmdRunner.AddCmdResult(setKey, fakes.FakeCmdResult{Error: fmt.Errorf("set boom %d", i)})
			}

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("set boom 3")))
			Expect(err).To(MatchError(ContainSubstring("after 3 attempts")))
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				setCmd(""),
				setCmd("~bosh."),
				setCmd("~bosh."),
				setCmd("~bosh."),
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second, 2 * time.Second}))
		})

		It("tries three times in total, sleeping one and then two seconds, and returns the last error", func() {
			for i := 1; i <= 3; i++ {
				fakeCmdRunner.AddCmdResult(setKey, fakes.FakeCmdResult{Error: fmt.Errorf("boom %d", i)})
			}

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("boom 3")))
			Expect(err).To(MatchError(ContainSubstring("after 3 attempts")))
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				setCmd("~bosh."),
				setCmd("~bosh."),
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second, 2 * time.Second}))
		})

		It("succeeds when a retry succeeds", func() {
			fakeCmdRunner.AddCmdResult(setKey, fakes.FakeCmdResult{Error: errors.New("boom")})
			readback("~bosh")

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				setCmd("~bosh."),
				readCmd,
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second}))
		})

		It("returns an error when the readback command fails", func() {
			fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{Error: errors.New("read boom"), Sticky: true})

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("read boom")))
			Expect(err).To(MatchError(ContainSubstring("after 3 attempts")))
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				readCmd,
				readCmd,
				readCmd,
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second, 2 * time.Second}))
		})

		It("retries a readback that fails once", func() {
			readKey := strings.Join(readCmd, " ")
			fakeCmdRunner.AddCmdResult(readKey, fakes.FakeCmdResult{Stdout: "Link 5 (bosh-dns): ~bosh\n"})
			fakeCmdRunner.AddCmdResult(readKey, fakes.FakeCmdResult{Error: errors.New("read boom")})
			fakeCmdRunner.AddCmdResult(readKey, fakes.FakeCmdResult{Stdout: "Link 5 (bosh-dns): ~bosh\n"})

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
				readCmd,
				setCmd("~bosh."),
				readCmd,
				readCmd,
			}))
			Expect(sleeps).To(Equal([]time.Duration{1 * time.Second}))
		})

		It("does not retry a readback that runs but does not match the plan", func() {
			readback("~other")

			err := mgr.UpdateDomains([]string{"bosh."})

			Expect(err).To(MatchError(ContainSubstring("does not hold")))
			Expect(fakeCmdRunner.RunCommands).To(HaveLen(3))
			Expect(sleeps).To(BeEmpty())
		})
	})

	Describe("readback verification", func() {
		DescribeTable("fails when systemd-resolved does not hold the planned set",
			func(output string, fragments ...string) {
				fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{Stdout: output, Sticky: true})

				err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns."})

				Expect(err).To(HaveOccurred())
				for _, fragment := range fragments {
					Expect(err).To(MatchError(ContainSubstring(fragment)))
				}
			},
			Entry("an entry is missing", "Link 5 (bosh-dns): ~bosh\n", `missing ["bosh-dns."]`),
			Entry("an entry is extra", "Link 5 (bosh-dns): ~bosh ~bosh-dns ~extra\n", `unexpected ["extra."]`),
			Entry("an entry is not route-only", "Link 5 (bosh-dns): ~bosh bosh-dns\n", `not routing-only ["bosh-dns"]`),
			Entry("the link is empty", "Link 5 (bosh-dns):\n", `missing ["bosh-dns." "bosh."]`),
			Entry("the output has no colon", "nothing useful", "unexpected resolvectl output"),
			Entry("a token is not a valid name", "Link 5 (bosh-dns): ~bosh ~bosh-dns ~a..b\n", "Parsing domain"),
		)

		It("accepts systemd's own spelling, without trailing dots and in another order", func() {
			readback("~bosh-dns", "~bosh")

			err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns."})

			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts spellings that only differ in case or escaping", func() {
			readback("~BOSH-DNS.", `~\098osh`)

			err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns."})

			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts a readback that wraps onto indented lines", func() {
			fakeCmdRunner.AddCmdResult(strings.Join(readCmd, " "), fakes.FakeCmdResult{
				Stdout: "Link 5 (bosh-dns): ~bosh\n        ~bosh-dns\n        ~other.example\n",
				Sticky: true,
			})

			err := mgr.UpdateDomains([]string{"bosh.", "bosh-dns.", "other.example."})

			Expect(err).NotTo(HaveOccurred())
		})

		It("expects an empty readback for an empty plan", func() {
			readback("~stale")

			err := mgr.UpdateDomains(nil)

			Expect(err).To(MatchError(ContainSubstring(`unexpected ["stale."]`)))
		})
	})
})

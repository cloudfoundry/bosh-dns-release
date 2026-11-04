package manager

import (
	"fmt"
	"sort"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/miekg/dns"
)

// MaxSystemdResolvedLinkDomains is systemd-resolved's per-link domain cap
// (LINK_SEARCH_DOMAINS_MAX).
const MaxSystemdResolvedLinkDomains = 256

const (
	systemdResolvedLogTag   = "SystemdResolvedManager"
	systemdResolvedLinkName = "bosh-dns"
	argumentListTooLong     = "Argument list too long"

	// systemdDomainMax is DNS_HOSTNAME_MAX, the longest name systemd accepts,
	// measured on its escaped spelling.
	systemdDomainMax = 253
)

var systemdResolvedRetryBackoff = []time.Duration{1 * time.Second, 2 * time.Second}

type systemdResolvedManager struct {
	cmdRunner system.CmdRunner
	logger    boshlog.Logger
	sleep     func(time.Duration)
}

func NewSystemdResolvedManager(cmdRunner system.CmdRunner, logger boshlog.Logger) systemdResolvedManager {
	return NewSystemdResolvedManagerWithSleeper(cmdRunner, logger, time.Sleep)
}

// NewSystemdResolvedManagerWithSleeper is NewSystemdResolvedManager with the
// function that waits between resolvectl retries supplied by the caller.
func NewSystemdResolvedManagerWithSleeper(cmdRunner system.CmdRunner, logger boshlog.Logger, sleep func(time.Duration)) systemdResolvedManager {
	return systemdResolvedManager{
		cmdRunner: cmdRunner,
		logger:    logger,
		sleep:     sleep,
	}
}

// UpdateDomains configures the covering set of the given domains on the
// bosh-dns link as routing-only domains: duplicates and every domain below
// another entry are dropped, since a routing domain also routes its
// subdomains. It returns an error without running resolvectl if the set still
// exceeds systemd-resolved's per-link limit, and it reads the link back after
// setting it to confirm that systemd-resolved holds exactly the planned set.
func (r systemdResolvedManager) UpdateDomains(domains []string) error {
	planned := r.planDomains(domains)
	if len(planned) > MaxSystemdResolvedLinkDomains {
		err := fmt.Errorf(
			"cannot configure systemd-resolved: %d domains are needed to cover the configured domains, but systemd-resolved allows at most %d domains per link",
			len(planned), MaxSystemdResolvedLinkDomains,
		)
		r.logger.Error(systemdResolvedLogTag, "%s", err)
		return err
	}

	args := make([]string, len(planned))
	for i, name := range planned {
		args[i] = "~" + systemdEscapedDomain(name)
	}
	if len(args) == 0 {
		// An empty argument clears the link; no argument at all would only print it.
		args = []string{""}
	}

	r.logger.Info(systemdResolvedLogTag, "Configuring %d routing domains from %d configured domains", len(planned), len(domains))

	err := r.clearPlainDomains()
	if err != nil {
		return err
	}

	err = r.applyDomains(args)
	if err != nil {
		return err
	}

	return r.verifyDomains(planned)
}

// planDomains returns the canonical, sorted, deduplicated names that cover the
// given domains, without any name that has a proper parent in the set. The
// root domain is kept but never prunes anything.
func (r systemdResolvedManager) planDomains(domains []string) []string {
	set := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		trimmed := strings.TrimPrefix(domain, "~")
		err := checkDecimalEscapes(trimmed)
		var name string
		if err == nil {
			name, err = canonicalDomain(trimmed)
		}
		if err != nil {
			r.logger.Warn(systemdResolvedLogTag, "Skipping invalid domain %q: %s", domain, err)
			continue
		}
		set[name] = struct{}{}
	}

	planned := make([]string, 0, len(set))
	for name := range set {
		if !hasProperParent(name, set) {
			planned = append(planned, name)
		}
	}
	sort.Strings(planned)

	if _, ok := set["."]; ok {
		r.logger.Warn(systemdResolvedLogTag, "The root domain is configured, so every query is routed to bosh-dns")
	}
	return planned
}

// checkDecimalEscapes rejects names with a backslash that is not followed by
// a character to escape, or a decimal escape that is not exactly three digits
// with a value of at most 255, because the packer wraps such values silently.
func checkDecimalEscapes(domain string) error {
	for i := 0; i < len(domain); i++ {
		if domain[i] != '\\' {
			continue
		}
		if i+1 >= len(domain) {
			return fmt.Errorf("the name ends with a lone backslash")
		}
		if !isDigit(domain[i+1]) {
			i++ // `\\`, `\.` and other escaped characters
			continue
		}
		if i+3 >= len(domain) || !isDigit(domain[i+2]) || !isDigit(domain[i+3]) {
			return fmt.Errorf("the decimal escape at offset %d needs exactly three digits", i)
		}
		value := int(domain[i+1]-'0')*100 + int(domain[i+2]-'0')*10 + int(domain[i+3]-'0')
		if value > 255 {
			return fmt.Errorf("the decimal escape %q at offset %d is above 255", domain[i:i+4], i)
		}
		i += 3
	}
	return nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// systemdEscapedDomain rewrites a canonical name so that every escape is one
// systemd accepts: `\.`, `\\` and `\DDD`. Any other escaped byte becomes `\DDD`.
func systemdEscapedDomain(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		if name[i] != '\\' || i+1 >= len(name) {
			b.WriteByte(name[i])
			continue
		}
		next := name[i+1]
		switch {
		case next == '.' || next == '\\':
			b.WriteString(name[i : i+2])
			i++
		case isDigit(next) && i+3 < len(name):
			b.WriteString(name[i : i+4])
			i += 3
		default:
			fmt.Fprintf(&b, "\\%03d", next)
			i++
		}
	}
	return b.String()
}

// canonicalDomain decodes escapes, folds ASCII case, and makes the name fully
// qualified, so equal names compare equal and label boundaries are exact.
func canonicalDomain(domain string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("the name is empty")
	}

	buf := make([]byte, 256)
	off, err := dns.PackDomainName(fqdn(escapeNonASCII(domain)), buf, 0, nil, false)
	if err != nil {
		return "", err
	}

	if n := systemdPresentationLength(buf[:off]); n > systemdDomainMax {
		return "", fmt.Errorf("the name is %d characters long when escaped for systemd-resolved, which allows at most %d", n, systemdDomainMax)
	}

	name, _, err := dns.UnpackDomainName(buf[:off], 0)
	if err != nil {
		return "", err
	}
	return dns.CanonicalName(name), nil
}

// fqdn appends a dot unless the name already ends in an unescaped one.
// dns.Fqdn counts runes, so a multibyte character before the backslashes
// flips its parity and drops the last label.
func fqdn(s string) string {
	if strings.HasSuffix(s, ".") {
		n := 0
		for i := len(s) - 2; i >= 0 && s[i] == '\\'; i-- {
			n++
		}
		if n%2 == 0 {
			return s
		}
	}
	return s + "."
}

// escapeNonASCII writes every byte above 0x7F as a decimal escape. The packer
// checks for the final dot with dns.IsFqdn, which has the same rune-counting
// flaw as dns.Fqdn, so a multibyte character before `\\` would make it reject
// or misread the name.
func escapeNonASCII(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && isDigit(s[i+1]):
			b.WriteString(s[i:min(i+4, len(s))])
			i += 3
		case s[i] == '\\' && i+1 < len(s) && s[i+1] >= 0x80:
			fmt.Fprintf(&b, "\\%03d", s[i+1])
			i++
		case s[i] == '\\' && i+1 < len(s):
			b.WriteString(s[i : i+2])
			i++
		case s[i] >= 0x80:
			fmt.Fprintf(&b, "\\%03d", s[i])
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// systemdPresentationLength is the length dns_name_concat would produce,
// which systemd caps at 253 (DNS_HOSTNAME_MAX).
func systemdPresentationLength(wire []byte) int {
	n, first := 0, true
	for off := 0; off < len(wire) && wire[off] != 0; {
		l := int(wire[off])
		if !first {
			n++
		}
		for _, b := range wire[off+1 : off+1+l] {
			switch {
			case b == '.' || b == '\\':
				n += 2
			case b == '_' || b == '-' || isDigit(b) || (b|0x20 >= 'a' && b|0x20 <= 'z'):
				n++
			default:
				n += 4
			}
		}
		first = false
		off += 1 + l
	}
	return n
}

// hasProperParent reports whether a name other than the given one in the set
// is a parent of it. It tests each suffix at a label boundary, which is what
// dns.IsSubDomain does against every member, in one lookup per label. The
// root never counts as a parent.
func hasProperParent(name string, set map[string]struct{}) bool {
	starts := dns.Split(name)
	for i := 1; i < len(starts); i++ {
		parent := name[starts[i]:]
		if _, ok := set[parent]; ok && dns.IsSubDomain(parent, name) {
			return true
		}
	}
	return false
}

// clearPlainDomains empties the link when it holds any domain that is not
// routing-only. systemd-resolved sets its changed flag only when a domain is
// added or removed, so flipping a reused domain to routing-only would not be
// persisted or reflected in resolv.conf. A failed read is not an error: the
// set that follows still runs.
func (r systemdResolvedManager) clearPlainDomains() error {
	stdout, _, _, err := r.cmdRunner.RunCommand("resolvectl", "domain", systemdResolvedLinkName)
	var tokens []string
	if err == nil {
		tokens, err = linkDomainTokens(stdout)
	}
	if err != nil {
		r.logger.Warn(systemdResolvedLogTag, "Could not read the domains of the %s link before setting them: %s", systemdResolvedLinkName, err)
		return nil
	}

	for _, token := range tokens {
		if !strings.HasPrefix(token, "~") {
			r.logger.Info(systemdResolvedLogTag, "The %s link holds plain domains, clearing it so that systemd-resolved records the routing-only state", systemdResolvedLinkName)
			_, _, err = r.runResolvectl([]string{"domain", systemdResolvedLinkName, ""}, "Clearing domains of bosh-dns link")
			return err
		}
	}
	return nil
}

func (r systemdResolvedManager) applyDomains(args []string) error {
	overflow, err := r.setDomains(args)
	if !overflow {
		return err
	}

	r.logger.Warn(systemdResolvedLogTag, "resolvectl reported %q, clearing the %s link before setting it again", argumentListTooLong, systemdResolvedLinkName)
	_, err = r.setDomains([]string{""})
	if err != nil {
		return bosherr.WrapError(err, "Clearing domains of bosh-dns link")
	}

	_, err = r.setDomains(args)
	return err
}

func (r systemdResolvedManager) setDomains(args []string) (overflow bool, err error) {
	resolvectlArgs := append([]string{"domain", systemdResolvedLinkName}, args...)
	_, overflow, err = r.runResolvectl(resolvectlArgs, "Setting domains on bosh-dns link")
	return overflow, err
}

// runResolvectl runs resolvectl, retrying failures with a backoff. It stops at
// once and reports overflow when resolvectl fails with E2BIG, because running
// the same command again cannot succeed.
func (r systemdResolvedManager) runResolvectl(args []string, action string) (stdout string, overflow bool, err error) {
	attempts := len(systemdResolvedRetryBackoff) + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		stdout, stderr, _, runErr := r.cmdRunner.RunCommand("resolvectl", args...)
		if runErr == nil {
			return stdout, false, nil
		}

		if strings.Contains(runErr.Error(), argumentListTooLong) ||
			strings.Contains(stderr, argumentListTooLong) ||
			strings.Contains(stdout, argumentListTooLong) {
			return "", true, bosherr.WrapError(runErr, action)
		}

		err = runErr
		if attempt < attempts {
			r.logger.Warn(systemdResolvedLogTag, "resolvectl failed (attempt %d of %d), retrying: %s", attempt, attempts, runErr)
			r.sleep(systemdResolvedRetryBackoff[attempt-1])
		}
	}

	return "", false, bosherr.WrapErrorf(err, "%s failed after %d attempts", action, attempts)
}

// linkDomainTokens returns the domains in the output of `resolvectl domain
// bosh-dns`, which reads `Link 5 (bosh-dns): ~bosh ~bosh-dns` and wraps long
// lists onto indented lines.
func linkDomainTokens(stdout string) ([]string, error) {
	colon := strings.Index(stdout, ":")
	if colon < 0 {
		return nil, fmt.Errorf("unexpected resolvectl output when reading domains of bosh-dns link: %q", stdout)
	}
	return strings.Fields(stdout[colon+1:]), nil
}

// verifyDomains reads the link back and compares canonical names, because
// systemd-resolved prints its own spelling of each domain.
func (r systemdResolvedManager) verifyDomains(planned []string) error {
	stdout, _, err := r.runResolvectl([]string{"domain", systemdResolvedLinkName}, "Reading domains of bosh-dns link")
	if err != nil {
		return err
	}

	tokens, err := linkDomainTokens(stdout)
	if err != nil {
		return err
	}

	actual := map[string]struct{}{}
	var notRouteOnly []string
	for _, token := range tokens {
		if !strings.HasPrefix(token, "~") {
			notRouteOnly = append(notRouteOnly, token)
		}
		name, err := canonicalDomain(strings.TrimPrefix(token, "~"))
		if err != nil {
			return bosherr.WrapErrorf(err, "Parsing domain %q read back from bosh-dns link", token)
		}
		actual[name] = struct{}{}
	}

	expected := make(map[string]struct{}, len(planned))
	for _, name := range planned {
		expected[name] = struct{}{}
	}

	var missing, extra []string
	for name := range expected {
		if _, ok := actual[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range actual {
		if _, ok := expected[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(notRouteOnly)

	if len(missing) == 0 && len(extra) == 0 && len(notRouteOnly) == 0 {
		return nil
	}

	err = fmt.Errorf(
		"systemd-resolved does not hold the configured domains on the bosh-dns link: missing %q, unexpected %q, not routing-only %q",
		missing, extra, notRouteOnly,
	)
	r.logger.Error(systemdResolvedLogTag, "%s", err)
	return err
}

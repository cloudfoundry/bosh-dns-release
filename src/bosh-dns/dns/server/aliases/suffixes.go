package aliases

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/miekg/dns"
)

const maxDomainLength = 253

var domainLabelPattern = regexp.MustCompile(`^[a-z0-9_]([a-z0-9_-]{0,61}[a-z0-9_])?$`)

// Suffixes maps alias hosts to the domain that systemd-resolved is
// configured with in their place.
type Suffixes struct {
	suffixes           map[string]string
	underscoreSuffixes map[string]string
}

// NewSuffixes returns a Suffixes with no entries that is ready to use.
func NewSuffixes() Suffixes {
	return Suffixes{
		suffixes:           map[string]string{},
		underscoreSuffixes: map[string]string{},
	}
}

// NewSuffixesFromMap returns an error for each entry it skips.
func NewSuffixesFromMap(load map[string]string) (Suffixes, []error) {
	suffixes := NewSuffixes()
	var errs []error

	for _, rawAlias := range slices.Sorted(maps.Keys(load)) {
		err := suffixes.set(rawAlias, load[rawAlias])
		if err != nil {
			errs = append(errs, bosherr.WrapErrorf(err, "alias %q", rawAlias))
		}
	}

	return suffixes, errs
}

// set records the suffix for an alias, where an alias starting with "_." is
// kept apart from one without it. It rejects an empty alias, an invalid alias
// or suffix, a suffix that is neither the alias nor a parent of it, and a
// second suffix that differs from the first. Repeating the same suffix is fine.
func (s Suffixes) set(rawAlias string, rawSuffix string) error {
	if rawAlias == "" {
		return bosherr.Error("empty alias")
	}

	suffix, err := CanonicalDomain(rawSuffix)
	if err != nil {
		return bosherr.WrapErrorf(err, "suffix %q", rawSuffix)
	}

	target := s.suffixes
	name := rawAlias
	if strings.HasPrefix(rawAlias, "_.") {
		target = s.underscoreSuffixes
		name = strings.TrimPrefix(rawAlias, "_.")
	}

	host, err := CanonicalDomain(name)
	if err != nil {
		return bosherr.WrapError(err, "invalid alias")
	}

	if !dns.IsSubDomain(suffix, host) {
		return bosherr.Errorf("suffix %q is not the alias or a parent of it", suffix)
	}

	if existing, found := target[host]; found {
		if existing == suffix {
			return nil
		}
		return bosherr.Errorf("the alias already has the suffix %q", existing)
	}

	target[host] = suffix
	return nil
}

// IsEmpty reports whether s holds no suffixes.
func (s Suffixes) IsEmpty() bool {
	return len(s.suffixes) == 0 && len(s.underscoreSuffixes) == 0
}

// DefinedIn also returns the sorted names of the aliases that c does not define.
func (s Suffixes) DefinedIn(c Config) (Suffixes, []string) {
	defined := NewSuffixes()
	undefined := []string{}

	for host, suffix := range s.suffixes {
		if _, found := c.aliases[host]; found {
			defined.suffixes[host] = suffix
		} else {
			undefined = append(undefined, host)
		}
	}

	for host, suffix := range s.underscoreSuffixes {
		if _, found := c.underscoreAliases[host]; found {
			defined.underscoreSuffixes[host] = suffix
		} else {
			undefined = append(undefined, "_."+host)
		}
	}

	slices.Sort(undefined)

	return defined, undefined
}

// CanonicalDomain returns raw lowercased and fully qualified.
func CanonicalDomain(raw string) (string, error) {
	if raw == "" {
		return "", bosherr.Error("empty domain")
	}

	if strings.IndexFunc(raw, func(r rune) bool { return r >= utf8.RuneSelf }) >= 0 {
		return "", bosherr.Error("only ASCII characters are allowed")
	}

	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return "", bosherr.Error("whitespace is not allowed")
	}

	if strings.HasPrefix(raw, "~") {
		return "", bosherr.Error("write it without the ~ prefix")
	}

	if strings.Contains(raw, "*") {
		return "", bosherr.Error("wildcards are not allowed")
	}

	domain := dns.CanonicalName(raw)
	if domain == "." {
		return "", bosherr.Error("the root domain is not allowed")
	}

	name := strings.TrimSuffix(domain, ".")
	if len(name) > maxDomainLength {
		return "", bosherr.Error("not a valid domain name")
	}

	for _, label := range strings.Split(name, ".") {
		if !domainLabelPattern.MatchString(label) {
			return "", bosherr.Error("not a valid domain name")
		}
	}

	return domain, nil
}

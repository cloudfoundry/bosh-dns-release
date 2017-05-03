package aliases

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"sort"
	"strings"
)

type Config struct {
	aliases           map[QualifiedName][]QualifiedName
	underscoreAliases map[QualifiedName][]QualifiedName
}

func NewConfig() Config {
	return Config{
		aliases:           map[QualifiedName][]QualifiedName{},
		underscoreAliases: map[QualifiedName][]QualifiedName{},
	}
}

func MustNewConfigFromMap(load map[string][]string) Config {
	config, err := NewConfigFromMap(load)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func NewConfigFromMap(load map[string][]string) (Config, error) {
	config := NewConfig()

	for alias, domains := range load {
		err := config.setAlias(alias, domains)
		if err != nil {
			return config, err
		}
	}

	return config, nil
}

func (c *Config) UnmarshalJSON(j []byte) error {
	primitive := map[string][]string{}

	err := json.Unmarshal(j, &primitive)
	if err != nil {
		return err
	}

	config, err := NewConfigFromMap(primitive)
	if err != nil {
		return err
	}

	*c = config
	return nil
}

func (c *Config) setAlias(alias string, domains []string) error {
	if alias == "" {
		return errors.New("bad alias format: empty alias qn")
	}

	qualifedDomains := []QualifiedName{}
	for _, domain := range domains {
		qualifedDomains = append(qualifedDomains, QualifiedName(dns.Fqdn(domain)))
	}

	if strings.HasPrefix(alias, "_.") {
		splitAlias := strings.SplitN(alias, ".", 2)
		c.underscoreAliases[QualifiedName(dns.Fqdn(splitAlias[1]))] = qualifedDomains
	} else {

		c.aliases[QualifiedName(dns.Fqdn(alias))] = qualifedDomains
	}

	return nil
}

func (c Config) IsReduced() bool {
	for _, domains := range c.aliases {
		for alias, _ := range c.aliases {
			for _, domain := range domains {
				if alias == domain {
					return false
				}
			}
		}
	}

	return true
}

func (c Config) Resolutions(maybeAlias QualifiedName) []QualifiedName {
	for alias, domains := range c.aliases {
		if alias == maybeAlias {
			return domains
		}
	}

	splitMaybeAlias := strings.SplitN(string(maybeAlias), ".", 2)
	if len(splitMaybeAlias) == 2 {
		for underscoreAlias, domains := range c.underscoreAliases {
			if string(underscoreAlias) != splitMaybeAlias[1] {
				continue
			}

			rewrittenDomains := []QualifiedName{}

			for _, domain := range domains {
				if strings.HasPrefix(string(domain), "_.") {
					splitDomain := strings.SplitN(string(domain), ".", 2)
					domain = QualifiedName(fmt.Sprintf("%s.%s", splitMaybeAlias[0], splitDomain[1]))
				}

				rewrittenDomains = append(rewrittenDomains, domain)
			}

			return rewrittenDomains
		}
	}

	return []QualifiedName{maybeAlias}
}

func (c Config) Merge(other Config) Config {
	for alias, targets := range other.aliases {
		if _, found := c.aliases[alias]; found {
			continue
		}

		c.aliases[alias] = targets
	}

	for alias, targets := range other.underscoreAliases {
		if _, found := c.underscoreAliases[alias]; found {
			continue
		}

		c.underscoreAliases[alias] = targets
	}

	return c
}

func (c Config) ReducedForm() (Config, error) {
	aliases := []string{}
	for alias, _ := range c.aliases {
		aliases = append(aliases, string(alias))
	}

	sort.Strings(aliases)

	for _, alias := range aliases {
		resolvedAlias, err := c.reduce2(QualifiedName(alias), 0)
		if err != nil {
			return Config{}, fmt.Errorf("failed to resolve %s: %s", alias, err)
		}

		c.aliases[QualifiedName(alias)] = resolvedAlias
	}

	return c, nil
}

func (c Config) reduce2(alias QualifiedName, depth int) ([]QualifiedName, error) {
	if depth > len(c.aliases)+1 {
		return nil, errors.New("recursion detected")
	}

	targets, found := c.aliases[alias]
	if !found {
		return []QualifiedName{alias}, nil
	}

	resolved := []QualifiedName{}

	for _, target := range targets {
		resolvedAlias, err := c.reduce2(target, depth+1)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, resolvedAlias...)
	}

	return resolved, nil
}

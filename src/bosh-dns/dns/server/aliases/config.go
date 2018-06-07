package aliases

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/miekg/dns"
)

type Config struct {
	aliases           map[string][]string
	underscoreAliases map[string][]string
	aliasHosts        []string
}

func NewConfig() Config {
	return Config{
		aliases:           map[string][]string{},
		underscoreAliases: map[string][]string{},
	}
}

func NewConfigFromMap(load map[string][]string) (Config, error) {
	config := NewConfig()

	for alias, domains := range load {
		err := config.SetAlias(alias, domains)
		if err != nil {
			return config, err
		}
	}

	config.aliasHosts = config.getAliasHosts()

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

func (c *Config) SetAlias(alias string, domains []string) error {
	if alias == "" {
		return errors.New("bad alias format: empty alias qn")
	}

	qualifedDomains := []string{}
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			qualifedDomains = append(qualifedDomains, dns.Fqdn(strings.Replace(dns.Fqdn(domain), "*", "q-s0", 1)))
		} else if net.ParseIP(domain) != nil {
			qualifedDomains = append(qualifedDomains, domain)
		} else {
			qualifedDomains = append(qualifedDomains, dns.Fqdn(domain))
		}
	}

	if strings.HasPrefix(alias, "_.") {
		splitAlias := strings.SplitN(alias, ".", 2)
		c.underscoreAliases[dns.Fqdn(splitAlias[1])] = qualifedDomains
	} else {
		c.aliases[dns.Fqdn(alias)] = qualifedDomains
	}

	c.aliasHosts = c.getAliasHosts()

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

func (c Config) Resolutions(maybeAlias string) []string {
	for alias, domains := range c.aliases {
		if alias == maybeAlias {
			return domains
		}
	}

	splitMaybeAlias := strings.SplitN(maybeAlias, ".", 2)
	if len(splitMaybeAlias) == 2 {
		for underscoreAlias, domains := range c.underscoreAliases {
			if underscoreAlias != splitMaybeAlias[1] {
				continue
			}

			rewrittenDomains := []string{}

			for _, domain := range domains {
				if strings.HasPrefix(domain, "_.") {
					splitDomain := strings.SplitN(domain, ".", 2)
					domain = fmt.Sprintf("%s.%s", splitMaybeAlias[0], splitDomain[1])
				}

				rewrittenDomains = append(rewrittenDomains, domain)
			}

			return rewrittenDomains
		}
	}

	return nil
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

	c.aliasHosts = c.getAliasHosts()

	return c
}

func (c Config) ReducedForm() (Config, error) {
	aliases := []string{}
	for alias, _ := range c.aliases {
		aliases = append(aliases, alias)
	}

	sort.Strings(aliases)

	for _, alias := range aliases {
		resolvedAlias, err := c.reduce2(alias, 0)
		if err != nil {
			return Config{}, fmt.Errorf("failed to resolve %s: %s", alias, err)
		}

		c.aliases[alias] = resolvedAlias
	}

	return c, nil
}

func (c Config) reduce2(alias string, depth int) ([]string, error) {
	if depth > len(c.aliases)+1 {
		return nil, errors.New("recursion detected")
	}

	targets, found := c.aliases[alias]
	if !found {
		return []string{alias}, nil
	}

	resolved := []string{}

	for _, target := range targets {
		resolvedAlias, err := c.reduce2(target, depth+1)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, resolvedAlias...)
	}

	return resolved, nil
}

func (c Config) AliasHosts() []string {
	return c.aliasHosts
}

func (c Config) getAliasHosts() []string {
	aliasHosts := []string{}
	allHosts := c.allAliasHosts()

	for host := range allHosts {
		foundParentDomain := false

		for comparisonHost := range allHosts {
			if comparisonHost != host && dns.IsSubDomain(comparisonHost, host) {
				foundParentDomain = true
				break
			}
		}

		if !foundParentDomain {
			aliasHosts = append(aliasHosts, dns.Fqdn(host))
		}
	}

	sort.Strings(aliasHosts)

	return aliasHosts
}

func (c Config) allAliasHosts() map[string]bool {
	allHosts := map[string]bool{}

	for host := range c.aliases {
		allHosts[host] = true
	}

	for host := range c.underscoreAliases {
		allHosts[host] = true
	}

	return allHosts
}

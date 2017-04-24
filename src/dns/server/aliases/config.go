package aliases

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"sort"
)

type Config map[QualifiedName][]QualifiedName

func (c Config) UnmarshalJSON(j []byte) error {
	primitive := map[string][]QualifiedName{}

	err := json.Unmarshal(j, &primitive)
	if err != nil {
		return err
	}

	//Go does not call custom unmarshal logic for type aliases to string
	//when used in the key segment of a map entry (though it does for values)
	for alias, domains := range primitive {
		if alias == "" {
			return errors.New("bad alias format: empty alias qn")
		}

		c[QualifiedName(dns.Fqdn(alias))] = domains
	}

	return nil
}

func (c Config) IsReduced() bool {
	for _, domains := range c {
		for alias, _ := range c {
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
	for alias, domains := range c {
		if alias == maybeAlias {
			return domains
		}
	}

	return []QualifiedName{maybeAlias}
}

func (c Config) Merge(other Config) Config {
	for alias, targets := range other {
		if _, found := c[alias]; found {
			continue
		}

		c[alias] = targets
	}

	return c
}

func (c Config) ReducedForm() (Config, error) {
	aliases := []string{}
	for alias, _ := range c {
		aliases = append(aliases, string(alias))
	}

	sort.Strings(aliases)

	for _, alias := range aliases {
		resolvedAlias, err := c.reduce2(QualifiedName(alias), 0)
		if err != nil {
			return Config{}, fmt.Errorf("failed to resolve %s: %s", alias, err)
		}

		c[QualifiedName(alias)] = resolvedAlias
	}

	return c, nil
}

func (c Config) reduce2(alias QualifiedName, depth int) ([]QualifiedName, error) {
	if depth > len(c)+1 {
		return nil, errors.New("recursion detected")
	}

	targets, found := c[alias]
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

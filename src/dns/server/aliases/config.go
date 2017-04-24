package aliases

import (
	"errors"
	"fmt"
	"sort"
)

type Config map[string][]string

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

func (c Config) Resolutions(maybeAlias string) []string {
	for alias, domains := range c {
		if alias == maybeAlias {
			return domains
		}
	}

	return []string{maybeAlias}
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
		aliases = append(aliases, alias)
	}

	sort.Strings(aliases)

	for _, alias := range aliases {
		resolvedAlias, err := c.reduce2(alias, 0)
		if err != nil {
			return Config{}, fmt.Errorf("failed to resolve %s: %s", alias, err)
		}

		c[alias] = resolvedAlias
	}

	return c, nil
}

func (c Config) reduce2(alias string, depth int) ([]string, error) {
	if depth > len(c)+1 {
		return nil, errors.New("recursion detected")
	}

	targets, found := c[alias]
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

package aliases_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"
	"testing"
)

func TestAliases(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/aliases")
}

func MustCastFromMap(from map[string][]string) aliases.Config {
	c := aliases.Config{}

	for alias, domains := range from {
		ds := []aliases.QualifiedName{}
		for _, domain := range domains {
			ds = append(ds, aliases.QualifiedName(domain))
		}
		c[aliases.QualifiedName(alias)] = ds
	}

	return c
}

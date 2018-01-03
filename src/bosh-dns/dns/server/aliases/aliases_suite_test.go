package aliases_test

import (
	"bosh-dns/dns/server/aliases"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAliases(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/aliases")
}

func MustNewConfigFromMap(load map[string][]string) aliases.Config {
	config, err := aliases.NewConfigFromMap(load)
	if err != nil {
		Fail(err.Error())
	}
	return config
}

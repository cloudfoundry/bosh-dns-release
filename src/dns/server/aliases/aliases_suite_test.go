package aliases_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAliases(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/aliases")
}


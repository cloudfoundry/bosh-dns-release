package addresses_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAddresses(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/config/addresses")
}

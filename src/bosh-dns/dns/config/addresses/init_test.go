package addresses_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAddresses(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/config/addresses")
}

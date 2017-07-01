package zone_pickers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestZonePickers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ZonePickers Suite")
}

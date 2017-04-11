package records_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRecords(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/records")
}

package records_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRecords(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/records")
}

package tracker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTracker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/tracker")
}

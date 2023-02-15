package shuffle_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestShuffle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/shuffle")
}

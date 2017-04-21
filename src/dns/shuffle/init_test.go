package shuffle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestShuffle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/shuffle")
}

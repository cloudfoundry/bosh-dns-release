package shuffle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dns-release/src/dns/shuffle"
)

var _ = Describe("Shuffle", func() {
	var (
		shuffler shuffle.Shuffle
	)
	BeforeEach(func() {
		shuffler = shuffle.New()
	})

	It("shuffles the given array", func() {
		src := []string{"1", "2", "3", "4"}

		Expect(shuffler.Shuffle(src)).To(ConsistOf("1", "2", "3", "4"))

		for i := 0; i < len(src); i++ {
			Eventually(func() string { return shuffler.Shuffle(src)[i] }).ShouldNot(Equal(src[i]))
		}
	})

	It("handles empty arrays", func() {
		Expect(shuffler.Shuffle(nil)).To(BeEmpty())
	})

	It("handle arrays of len 1", func() {
		src := []string{"1"}
		Expect(shuffler.Shuffle(src)).To(Equal(src))
	})
})

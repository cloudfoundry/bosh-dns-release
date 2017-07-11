package shuffle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/shuffle"
)

var _ = Describe("Shuffle", func() {
	var (
		shuffler shuffle.StringShuffle
	)
	BeforeEach(func() {
		shuffler = shuffle.NewStringShuffler()
	})

	It("shuffles the given array", func() {
		src := []string{
			"127.0.0.1",
			"127.0.0.2",
			"127.0.0.3",
			"127.0.0.4",
		}

		Expect(shuffler.Shuffle(src)).To(ConsistOf(src[0], src[1], src[2], src[3]))

		for i := 0; i < len(src); i++ {
			Eventually(func() string { return shuffler.Shuffle(src)[i] }).ShouldNot(Equal(src[i]))
		}
	})

	It("handles empty arrays", func() {
		Expect(shuffler.Shuffle(nil)).To(BeEmpty())
	})

	It("handle arrays of len 1", func() {
		src := []string{"127.0.0.1"}
		Expect(shuffler.Shuffle(src)).To(Equal(src))
	})
})

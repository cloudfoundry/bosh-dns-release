package shuffle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dns-release/src/dns/shuffle"
	"github.com/miekg/dns"
	"net"
)

var _ = Describe("Shuffle", func() {
	var (
		shuffler shuffle.AnswerShuffle
	)
	BeforeEach(func() {
		shuffler = shuffle.New()
	})

	It("shuffles the given array", func() {
		src := []dns.RR{
			&dns.A{A: net.IPv4(127, 0, 0, 1)},
			&dns.A{A: net.IPv4(127, 0, 0, 2)},
			&dns.A{A: net.IPv4(127, 0, 0, 3)},
			&dns.A{A: net.IPv4(127, 0, 0, 4)},
		}

		Expect(shuffler.Shuffle(src)).To(ConsistOf(src[0], src[1], src[2], src[3]))

		for i := 0; i < len(src); i++ {
			Eventually(func() dns.RR { return shuffler.Shuffle(src)[i] }).ShouldNot(Equal(src[i]))
		}
	})

	It("handles empty arrays", func() {
		Expect(shuffler.Shuffle(nil)).To(BeEmpty())
	})

	It("handle arrays of len 1", func() {
		src := []dns.RR{&dns.A{A: net.IPv4(127, 0, 0, 1)}}
		Expect(shuffler.Shuffle(src)).To(Equal(src))
	})
})

package shuffle

import (
	mathrand "math/rand"
	"time"
	"github.com/miekg/dns"
)

func init() {
	mathrand.Seed(time.Now().UTC().UnixNano())
}

type AnswerShuffle struct{}

func New() AnswerShuffle {
	return AnswerShuffle{}
}

func (s AnswerShuffle) Shuffle(src []dns.RR) []dns.RR {
	dst := make([]dns.RR, len(src))
	copy(dst, src)

	for i := len(src) - 1; i > 0; i-- {
		j := mathrand.Intn(i + 1)
		dst[i], dst[j] = dst[j], dst[i]
	}

	return dst
}

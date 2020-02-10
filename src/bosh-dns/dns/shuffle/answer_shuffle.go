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
	srccopy := make([]dns.RR, len(src))
	copy(srccopy, src)
	dst := make([]dns.RR, len(srccopy))

	for i := 0; i < len(dst); i++ {
		j := mathrand.Intn(len(srccopy))
		answer := srccopy[j]
		srccopy = s.remove(j, srccopy)
		dst[i] = answer
	}

	return dst
}

func (s AnswerShuffle) remove(index int, recs []dns.RR) []dns.RR {
	copy(recs[index:], recs[index+1:]) // left shift
	recs = recs[:len(recs)-1] // truncate
	return recs
}

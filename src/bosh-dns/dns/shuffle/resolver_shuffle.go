package shuffle

import (
	mathrand "math/rand"
	"time"
)

func init() {
	mathrand.Seed(time.Now().UTC().UnixNano())
}

type StringShuffle struct{}

func NewStringShuffler() StringShuffle {
	return StringShuffle{}
}

func (s StringShuffle) Shuffle(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)

	for i := len(src) - 1; i > 0; i-- {
		j := mathrand.Intn(i + 1)
		dst[i], dst[j] = dst[j], dst[i]
	}

	return dst
}

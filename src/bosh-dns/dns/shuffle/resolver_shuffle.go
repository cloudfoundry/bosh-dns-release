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
	srccopy := make([]string, len(src))
	copy(srccopy, src)
	dst := make([]string, len(srccopy))

	for i := 0; i < len(dst); i++ {
		j := mathrand.Intn(len(srccopy))
		answer := srccopy[j]
		srccopy = s.remove(j, srccopy)
		dst[i] = answer
	}

	return dst
}

func (s StringShuffle) remove(index int, strs []string) []string {
	copy(strs[index:], strs[index+1:]) // left shift
	strs = strs[:len(strs)-1]          // truncate
	return strs
}

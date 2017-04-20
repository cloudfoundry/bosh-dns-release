package crypto

import (
	"fmt"
	"errors"
)

type multipleDigestImpl struct {
	digests []Digest
}

func (m multipleDigestImpl) Verify(digest Digest) error {
	for _, candidateDigest := range m.digests {
		if candidateDigest.Algorithm() == digest.Algorithm() {
			return candidateDigest.Verify(digest)
		}
	}

	return errors.New(fmt.Sprintf("No digest found that matches %s", digest.Algorithm()))
}

func NewMultipleDigest(digests ...Digest) multipleDigestImpl {
	return multipleDigestImpl{digests: digests}
}

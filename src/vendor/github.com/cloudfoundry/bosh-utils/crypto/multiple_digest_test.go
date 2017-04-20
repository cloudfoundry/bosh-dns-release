package crypto_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("MultipleDigest", func() {
	var (
		expectedDigest VerifyingDigest
		digest1 Digest
		digest2 Digest
	)

	BeforeEach(func() {
		digest1 = NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
		digest2 = NewDigest(DigestAlgorithmSHA256, "07e1306432667f916639d47481edc4f2ca456454")

		expectedDigest = NewMultipleDigest(digest1, digest2)
	})

	It("should select the highest algo and verify that digest", func() {
		actualDigest := NewDigest(DigestAlgorithmSHA256, "07e1306432667f916639d47481edc4f2ca456454")

		err := expectedDigest.Verify(actualDigest)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should work with only a single digest", func() {
		expectedDigest := NewMultipleDigest(digest1)
		actualDigest := NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")

		err := expectedDigest.Verify(actualDigest)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should throw an error if the digest does not match", func() {
		expectedDigest := NewMultipleDigest(digest1, digest2)
		actualDigest := NewDigest(DigestAlgorithmSHA256, "b1e66f505465c28d705cf587b041a6506cfe749f")

		err := expectedDigest.Verify(actualDigest)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Expected sha256 digest \"07e1306432667f916639d47481edc4f2ca456454\" but received \"b1e66f505465c28d705cf587b041a6506cfe749f\""))
	})

	It("should throw an error if the algorithms do not match", func() {
		expectedDigest := NewMultipleDigest(digest1, digest2)
		actualDigest := NewDigest(DigestAlgorithmSHA512, "07e1306432667f916639d47481edc4f2ca456454")

		err := expectedDigest.Verify(actualDigest)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("No digest found that matches sha512"))
	})
})


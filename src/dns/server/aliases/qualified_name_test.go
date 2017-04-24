package aliases_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/aliases"

	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("QualifiedName", func() {
	var name QualifiedName

	BeforeEach(func() {
		name = QualifiedName("")
	})

	Describe("UnmarshalJSON", func() {
		It("unmarshals a string into its string field", func() {
			err := json.Unmarshal([]byte(`"one.domain."`), &name)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(name)).To(Equal("one.domain."))
		})

		It("must not be empty", func() {
			err := json.Unmarshal([]byte(`""`), &name)
			Expect(err).To(HaveOccurred())
		})

		It("adds a trailing dot if it needs to", func() {
			err := json.Unmarshal([]byte(`"alias"`), &name)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(name)).To(Equal("alias."))
		})
	})
})

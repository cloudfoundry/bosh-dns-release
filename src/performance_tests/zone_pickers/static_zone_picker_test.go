package zone_pickers_test

import (
	. "performance_tests/zone_pickers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticZonePicker", func() {
	Describe("NextZone", func() {
		It("always returns the string it was given", func() {
			picker := NewStaticZonePicker("some.domain.")

			Expect(picker.NextZone()).To(Equal("some.domain."))
			Expect(picker.NextZone()).To(Equal("some.domain."))
			Expect(picker.NextZone()).To(Equal("some.domain."))
		})
	})
})

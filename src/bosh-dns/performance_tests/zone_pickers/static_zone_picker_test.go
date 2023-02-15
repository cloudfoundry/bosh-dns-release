package zone_pickers_test

import (
	"bosh-dns/performance_tests/zone_pickers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticZonePicker", func() {
	Describe("NextZone", func() {
		It("always returns the string it was given", func() {
			picker := zone_pickers.NewStaticZonePicker("some.domain.")

			Expect(picker.NextZone()).To(Equal("some.domain."))
			Expect(picker.NextZone()).To(Equal("some.domain."))
			Expect(picker.NextZone()).To(Equal("some.domain."))
		})
	})
})

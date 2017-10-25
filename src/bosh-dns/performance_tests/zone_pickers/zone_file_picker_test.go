package zone_pickers_test

import (
	"bosh-dns/performance_tests/zone_pickers"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ZoneFilePicker", func() {
	var (
		sourceFile string
		picker     *zone_pickers.ZoneFilePicker
	)

	Describe("NewZoneFilePickerFromFile", func() {
		Context("when the given file is present", func() {
			BeforeEach(func() {
				file, err := ioutil.TempFile("", "dns_zone_data")
				Expect(err).ToNot(HaveOccurred())

				zoneContents := []byte(`{"zones":["1.domain.","2.domain.","3.domain."]}`)
				_, err = file.Write(zoneContents)
				Expect(err).ToNot(HaveOccurred())

				sourceFile = file.Name()
			})

			AfterEach(func() {
				os.Remove(sourceFile)
			})

			It("returns a pointer to a ZoneFilePicker", func() {
				picker, err := zone_pickers.NewZoneFilePickerFromFile(sourceFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(picker).ToNot(BeNil())
			})
		})

		Context("when the given file is NOT present", func() {
			It("returns a nil pointer and an error", func() {
				picker, err := zone_pickers.NewZoneFilePickerFromFile(sourceFile)
				Expect(picker).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Creating zone picker"))
			})
		})
	})

	Describe("NextZone", func() {
		Context("when the specified file is correctly populated", func() {
			BeforeEach(func() {
				file, err := ioutil.TempFile("", "dns_zone_data")
				Expect(err).ToNot(HaveOccurred())

				zoneContents := []byte(`{"zones":["1.domain.","2.domain.","3.domain."]}`)
				_, err = file.Write(zoneContents)
				Expect(err).ToNot(HaveOccurred())

				sourceFile = file.Name()
				picker, err = zone_pickers.NewZoneFilePickerFromFile(sourceFile)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				os.Remove(sourceFile)
			})

			It("round-robins through the list on each call", func() {
				zone1 := picker.NextZone()
				zone2 := picker.NextZone()
				zone3 := picker.NextZone()
				zone4 := picker.NextZone()

				Expect(zone1).To(Equal("1.domain."))
				Expect(zone2).To(Equal("2.domain."))
				Expect(zone3).To(Equal("3.domain."))
				Expect(zone4).To(Equal("1.domain."))
			})
		})

		Context("when the specified file has only one zone", func() {
			BeforeEach(func() {
				file, err := ioutil.TempFile("", "dns_zone_data")
				Expect(err).ToNot(HaveOccurred())

				zoneContents := []byte(`{"zones":["1.domain."]}`)
				_, err = file.Write(zoneContents)
				Expect(err).ToNot(HaveOccurred())

				sourceFile = file.Name()
				picker, err = zone_pickers.NewZoneFilePickerFromFile(sourceFile)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				os.Remove(sourceFile)
			})

			It("round-robins through the list on each call", func() {
				zone1 := picker.NextZone()
				zone2 := picker.NextZone()
				zone3 := picker.NextZone()
				zone4 := picker.NextZone()

				Expect(zone1).To(Equal("1.domain."))
				Expect(zone2).To(Equal("1.domain."))
				Expect(zone3).To(Equal("1.domain."))
				Expect(zone4).To(Equal("1.domain."))
			})
		})
	})
})

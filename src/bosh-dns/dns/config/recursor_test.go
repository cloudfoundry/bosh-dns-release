package config_test

import (
	"bosh-dns/dns/config"
	"bosh-dns/dns/config/configfakes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recursor", func() {
	var dnsConfig config.Config
	var resolvConfReader *configfakes.FakeRecursorReader
	var stringShuffler *configfakes.FakeStringShuffler

	BeforeEach(func() {
		dnsConfig = config.Config{}
		resolvConfReader = &configfakes.FakeRecursorReader{}
		stringShuffler = &configfakes.FakeStringShuffler{}
		stringShuffler.ShuffleStub = func(src []string) []string {
			return src
		}
	})

	Context("when dns config does not have any recursors configured", func() {
		BeforeEach(func() {
			resolvConfReader.GetReturns([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}, nil)
		})

		It("should generate recursors from the resolv.conf, shuffled", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
			Expect(stringShuffler.ShuffleCallCount()).To(Equal(1))

			Expect(resolvConfReader.GetCallCount()).To(Equal(1))
		})

		Context("when unable to Get fails on RecursorReader", func() {
			BeforeEach(func() {
				resolvConfReader.GetReturns([]string{"some-recursor-1:53"}, errors.New("some-error"))
			})

			It("should return the error", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(errors.New("some-error")))
			})
		})

		Context("when excluding recursors", func() {
			BeforeEach(func() {
				dnsConfig.ExcludedRecursors = []string{"some-recursor-1:53", "recursor-custom:1234"}
			})

			It("should exclude the recursor", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(stringShuffler.ShuffleCallCount()).To(Equal(1))
				Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2:53"}))
			})
		})
	})

	Context("when dns config does has recursors configured", func() {
		BeforeEach(func() {
			dnsConfig = config.Config{
				Recursors: []string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"},
			}
		})

		It("should shuffle the recursors", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(stringShuffler.ShuffleCallCount()).To(Equal(1))
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
		})

		Context("when excluding recursors", func() {
			BeforeEach(func() {
				dnsConfig.ExcludedRecursors = []string{"some-recursor-1:53", "recursor-custom:1234"}
			})

			It("should exclude the recursor", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(stringShuffler.ShuffleCallCount()).To(Equal(1))
				Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2:53"}))
			})
		})
	})

	Context("when dns config is not provided", func() {
		It("should not error", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

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
			return []string{src[1], src[0]}
		}
	})

	Context("when dnsConfig does not have any recursors configured", func() {
		BeforeEach(func() {
			resolvConfReader.GetReturns([]string{"some-recursor-1", "some-recursor-2"}, nil)
		})

		It("should generate recursors from the resolv.conf, shuffled", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2", "some-recursor-1"}))

			Expect(resolvConfReader.GetCallCount()).To(Equal(1))
		})

		Context("when unable to Get fails on RecursorReader", func() {
			BeforeEach(func() {
				resolvConfReader.GetReturns([]string{"some-recursor-1"}, errors.New("some-error"))
			})

			It("should return the error", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(errors.New("some-error")))
			})
		})
	})

	Context("when dnsConfig does has recursors configured", func() {
		BeforeEach(func() {
			dnsConfig = config.Config{
				Recursors: []string{"some-recursor-1", "some-recursor-2"},
			}
		})

		It("should shuffle the recursors", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2", "some-recursor-1"}))
		})
	})

	Context("when dnsConfig is not provided", func() {
		It("should not error", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

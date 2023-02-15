package config_test

import (
	"bosh-dns/dns/config"
	"bosh-dns/dns/config/configfakes"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recursor", func() {
	var dnsConfig config.Config
	var resolvConfReader *configfakes.FakeRecursorReader
	var stringShuffler *configfakes.FakeStringShuffler

	BeforeEach(func() {
		dnsConfig = config.NewDefaultConfig()
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

		It("should set the recursors from the resolv.conf file", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
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
	})

	Context("recursor_selection", func() {
		Context("serial", func() {
			BeforeEach(func() {
				dnsConfig.RecursorSelection = "serial"
				dnsConfig.Recursors = []string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}
			})

			It("should not shuffle the recursors", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(stringShuffler.ShuffleCallCount()).To(Equal(0))
				Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
			})
		})

		Context("smart", func() {
			BeforeEach(func() {
				dnsConfig.RecursorSelection = "smart"
				dnsConfig.Recursors = []string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}

				stringShuffler.ShuffleStub = func(src []string) []string {
					return []string{"shuffled"}
				}
			})

			It("should shuffle the recursors", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(stringShuffler.ShuffleCallCount()).To(Equal(1))
				Expect(dnsConfig.Recursors).Should(Equal([]string{"shuffled"}))
			})
		})

		Context("invalid value", func() {
			BeforeEach(func() {
				dnsConfig.RecursorSelection = "wrong"
			})

			It("should return an error", func() {
				err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
				Expect(err).To(MatchError("invalid value for recursor selection: 'wrong'"))
			})
		})
	})

	Context("excluding recursors", func() {
		BeforeEach(func() {
			dnsConfig.Recursors = []string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}
			dnsConfig.ExcludedRecursors = []string{"some-recursor-1:53", "recursor-custom:1234"}
		})

		It("should exclude the recursor", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2:53"}))
		})
	})

	Context("when dns config is not provided", func() {
		It("should not error", func() {
			err := config.ConfigureRecursors(resolvConfReader, stringShuffler, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

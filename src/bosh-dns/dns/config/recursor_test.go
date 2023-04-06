package config_test

import (
	"errors"

	"bosh-dns/dns/config"
	"bosh-dns/dns/config/configfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recursor", func() {
	var dnsConfig config.Config
	var resolvConfReader *configfakes.FakeRecursorReader

	BeforeEach(func() {
		dnsConfig = config.NewDefaultConfig()
		resolvConfReader = &configfakes.FakeRecursorReader{}
	})

	Context("when dns config does not have any recursors configured", func() {
		BeforeEach(func() {
			resolvConfReader.GetReturns([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}, nil)
		})

		It("should set the recursors from the resolv.conf file", func() {
			err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(ConsistOf([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
		})

		Context("when unable to Get fails on RecursorReader", func() {
			BeforeEach(func() {
				resolvConfReader.GetReturns([]string{"some-recursor-1:53"}, errors.New("some-error"))
			})

			It("should return the error", func() {
				err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)

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
				err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}))
			})
		})

		Context("smart", func() {
			var originalRecursors []string
			BeforeEach(func() {
				originalRecursors = []string{"some-recursor-1:53", "some-recursor-2:53", "recursor-custom:1234"}
				dnsConfig.RecursorSelection = "smart"
				dnsConfig.Recursors = originalRecursors
			})

			It("should shuffle the recursors", func() {
				err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(dnsConfig.Recursors).Should(ConsistOf(originalRecursors))
			})
		})

		Context("invalid value", func() {
			BeforeEach(func() {
				dnsConfig.RecursorSelection = "wrong"
			})

			It("should return an error", func() {
				err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
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
			err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(Equal([]string{"some-recursor-2:53"}))
		})
	})

	Context("when dns config is not provided", func() {
		It("should not error", func() {
			err := config.ConfigureRecursors(resolvConfReader, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

package config_test

import (
	"dns/config"
	"dns/config/configfakes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recursor", func() {
	var dnsConfig config.Config
	var resolvConfReader *configfakes.FakeRecursorReader

	BeforeEach(func() {
		dnsConfig = config.Config{}
		resolvConfReader = &configfakes.FakeRecursorReader{}
	})

	Context("when dnsConfig does not have any recursors configured", func() {
		BeforeEach(func() {
			resolvConfReader.GetReturns([]string{"some-recursor-1"}, nil)
		})

		It("should generate recursors from the resolv.conf", func() {
			err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(ConsistOf("some-recursor-1"))

			Expect(resolvConfReader.GetCallCount()).To(Equal(1))
		})

		Context("when unable to Get fails on RecursorReader", func() {
			BeforeEach(func() {
				resolvConfReader.GetReturns([]string{"some-recursor-1"}, errors.New("some-error"))
			})

			It("should return the error", func() {
				err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(errors.New("some-error")))
			})
		})

	})

	Context("when dnsConfig does has recursors configured", func() {

		BeforeEach(func() {
			dnsConfig = config.Config{
				Recursors: []string{"foo-bar"},
			}
		})

		It("should not modify config", func() {
			err := config.ConfigureRecursors(resolvConfReader, &dnsConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsConfig.Recursors).Should(ConsistOf("foo-bar"))
		})

	})

	Context("when dnsConfig is not provided", func() {
		It("should not error", func() {
			err := config.ConfigureRecursors(resolvConfReader, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

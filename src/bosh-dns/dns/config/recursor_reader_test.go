package config_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/config"
	"bosh-dns/dns/manager/managerfakes"
)

var _ = Describe("RecursorReader", func() {
	var (
		recursorReader       RecursorReader
		dnsManager           *managerfakes.FakeDNSManager
		dnsServerDomainNames []string
		loopackAddress       string
	)

	BeforeEach(func() {
		dnsServerDomainNames = []string{"dns-server-hostname-1", "dns-server-hostname-2"}
		loopackAddress = "127.0.0.1"
		dnsManager = new(managerfakes.FakeDNSManager)
		recursorReader = NewRecursorReader(dnsManager, dnsServerDomainNames)
	})

	Context("when there are no dns servers", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{}, nil)
		})

		It("returns an empty array", func() {
			recursors, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(recursors).To(HaveLen(0))
		})
	})

	Context("when only the DNS server addresses are configured", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns(dnsServerDomainNames, nil)
		})

		It("returns an empty array", func() {
			recursors, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(recursors).To(HaveLen(0))
		})
	})

	Context("when manager has only the loopback address", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{loopackAddress}, nil)
		})

		It("returns an empty array", func() {
			nameservers, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(nameservers).To(HaveLen(0))
		})
	})

	Context("when manager.Read returns empty string results", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{"", "10.0.0.1", "10.0.0.2"}, nil)
		})

		It("returns only non-empty strings`", func() {
			nameservers, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(nameservers).To(HaveLen(2))
			Expect(nameservers).To(ConsistOf("10.0.0.1:53", "10.0.0.2:53"))
		})
	})

	Context("when multiple recursors are configured", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns(append(dnsServerDomainNames, "189.8.0.9", "189.8.0.10", "189.10.10.10:1234"), nil)
		})

		It("returns all entries except the DNS server itself", func() {
			recursors, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(recursors).To(HaveLen(3))
			Expect(recursors).To(ConsistOf("189.8.0.9:53", "189.8.0.10:53", "189.10.10.10:1234"))
		})
	})

	Context("when reading configuration errors", func() {
		var readErr error

		BeforeEach(func() {
			readErr = errors.New("did you want a doughnut?")
			dnsManager.ReadReturns([]string{}, readErr)
		})

		It("returns back the error", func() {
			_, err := recursorReader.Get()

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(readErr))
		})
	})
})

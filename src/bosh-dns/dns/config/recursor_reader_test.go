package config_test

import (
	. "bosh-dns/dns/config"
	"bosh-dns/dns/manager/managerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
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
			dnsManager.ReadReturns([]string{"", "10.0.0.1", "my.dns.server"}, nil)
		})

		It("returns only non-empty strings`", func() {
			nameservers, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(nameservers).To(HaveLen(2))
			Expect(nameservers).To(ConsistOf("10.0.0.1:53", "my.dns.server:53"))
		})
	})

	Context("when multiple recursors are configured", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns(append(dnsServerDomainNames, "recursor-1", "recursor-2"), nil)
		})

		It("returns all entries except the DNS server itself", func() {
			recursors, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(recursors).To(HaveLen(2))
			Expect(recursors).To(ConsistOf("recursor-1:53", "recursor-2:53"))
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

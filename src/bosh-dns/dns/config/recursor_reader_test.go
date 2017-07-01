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
		recursorReader      RecursorReader
		dnsManager          *managerfakes.FakeDNSManager
		dnsServerDomainName string
		loopackAddress      string
	)

	BeforeEach(func() {
		dnsServerDomainName = "dns-server-hostname"
		loopackAddress = "127.0.0.1"
		dnsManager = new(managerfakes.FakeDNSManager)
		recursorReader = NewRecursorReader(dnsManager, dnsServerDomainName)
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

	Context("when only the DNS server address is configured", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{dnsServerDomainName}, nil)
		})

		It("returns an empty array", func() {
			recursors, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(recursors).To(HaveLen(0))
		})
	})

	Context("When resolv.conf has only the loopback address", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{loopackAddress}, nil)
		})

		It("returns an empty array", func() {
			nameservers, err := recursorReader.Get()

			Expect(err).ToNot(HaveOccurred())
			Expect(nameservers).To(HaveLen(0))
		})
	})

	Context("when multiple recursors are configured", func() {
		BeforeEach(func() {
			dnsManager.ReadReturns([]string{"recursor-1", dnsServerDomainName, "recursor-2"}, nil)
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

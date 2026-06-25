package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSystemdResolvedUpdater(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemdResolvedUpdater Suite")
}

var _ = Describe("parentDomains", func() {
	It("extracts the parent domain (drops the leftmost label) from each upcheck domain", func() {
		Expect(parentDomains([]string{"upcheck.bosh-dns.", "instance.health.bosh."})).To(ConsistOf("bosh-dns.", "health.bosh."))
	})

	It("handles multiple upcheck domains", func() {
		Expect(parentDomains([]string{"upcheck.bosh-dns.", "health.bosh."})).To(ConsistOf("bosh-dns.", "bosh."))
	})

	It("ignores domains with no dot or a trailing dot only", func() {
		Expect(parentDomains([]string{"bosh-dns.", "bosh-dns"})).To(BeEmpty())
	})

	It("returns empty when given no domains", func() {
		Expect(parentDomains([]string{})).To(BeEmpty())
	})
})

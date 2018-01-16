package criteria_test

import (
	"bosh-dns/dns/server/criteria"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseSegment", func() {
	It("creates segment from long-form fqdn", func() {
		s, err := criteria.ParseSegment("q-s0.ig.net.depl.bosh", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.Segment{
			Query:      "q-s0",
			Group:      "",
			Instance:   "ig",
			Network:    "net",
			Deployment: "depl",
			Domain:     "bosh",
		}))
	})

	It("creates segment from short-form fqdn", func() {
		s, err := criteria.ParseSegment("q-short.q-group.bosh", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.Segment{
			Query:      "q-short",
			Group:      "q-group",
			Instance:   "",
			Network:    "",
			Deployment: "",
			Domain:     "bosh",
		}))
	})

	It("errors when the fqdn cannot be split into a query and group segment", func() {
		_, err := criteria.ParseSegment("garbage", []string{"bosh"})
		Expect(err).To(MatchError("domain is malformed"))
	})

	It("ignores the domain when tld is not in the list of domains", func() {
		s, err := criteria.ParseSegment("garbage.potato", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.Segment{
			Query:      "garbage",
			Group:      "potato",
			Instance:   "",
			Network:    "",
			Deployment: "",
			Domain:     "",
		}))
	})

	It("errors when the number of group segments is 2", func() {
		_, err := criteria.ParseSegment("flying.fly.away.bosh", []string{"bosh"})
		Expect(err).To(MatchError(`bad group segment query had 2 values []string{"fly", "away"}`))
	})

	It("errors when the number of group segments > 3", func() {
		_, err := criteria.ParseSegment("query.one.two.three.four.bosh", []string{"bosh"})
		Expect(err).To(MatchError(`bad group segment query had 4 values []string{"one", "two", "three", "four"}`))
	})
})

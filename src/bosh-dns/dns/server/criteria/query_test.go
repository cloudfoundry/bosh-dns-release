package criteria_test

import (
	"bosh-dns/dns/server/criteria"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseQuery", func() {
	It("parses long-form", func() {
		s, err := criteria.ParseQuery("q-s0.ig.net.depl.bosh", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.NewLongFormQuery("q-s0", "ig", "bosh", "", "net", "depl")))
	})

	It("parses short-form", func() {
		s, err := criteria.ParseQuery("q-short.q-group.bosh", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.NewShortFormQuery("q-short", "", "q-group", "bosh")))
	})

	It("errors when the fqdn cannot be split into a query and group segment", func() {
		_, err := criteria.ParseQuery("garbage", []string{"bosh"})
		Expect(err).To(MatchError("domain is malformed"))
	})

	It("creates an agent_id form when given an agent_id", func() {
		s, err := criteria.ParseQuery("agent_id.bosh-agent-id.", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.NewAgentIDFormQuery("agent_id")))
	})

	It("ignores the domain when tld is not in the list of domains", func() {
		s, err := criteria.ParseQuery("garbage.potato", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.NewNonBoshDNSQuery("garbage")))
	})

	It("errors when the number of group segments is 2", func() {
		_, err := criteria.ParseQuery("flying.fly.away.bosh", []string{"bosh"})
		Expect(err).To(MatchError(`bad group segment query had 2 values []string{"fly", "away"}`))
	})

	It("errors when the number of group segments > 3", func() {
		_, err := criteria.ParseQuery("query.one.two.three.four.bosh", []string{"bosh"})
		Expect(err).To(MatchError(`bad group segment query had 4 values []string{"one", "two", "three", "four"}`))
	})

	It("garbage", func() {
		s, err := criteria.ParseQuery("q-s0m1.my-group.my-network.my-deployment.bosh", []string{"bosh"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(criteria.NewLongFormQuery("q-s0m1", "my-group", "bosh", "", "my-network", "my-deployment")))
	})
})

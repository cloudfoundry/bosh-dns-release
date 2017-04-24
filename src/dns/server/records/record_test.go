package records_test

import (
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fqdn", func() {
	var record records.Record

	BeforeEach(func() {
		record = records.Record{
			Id:         "label",
			Group:      "group",
			Network:    "network",
			Deployment: "deployment",
			Ip:         "ip",
		}
	})

	Context("when including job label", func() {
		It("returns an fqdn with job label prepended", func() {
			Expect(record.Fqdn(true)).To(Equal("label.group.network.deployment.bosh."))
		})
	})

	Context("when not including job label", func() {
		It("returns an fqdn without job label", func() {
			Expect(record.Fqdn(false)).To(Equal("group.network.deployment.bosh."))
		})
	})
})

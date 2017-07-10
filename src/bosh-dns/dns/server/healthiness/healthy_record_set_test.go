package healthiness_test

import (
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/records"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthyRecordSet", func() {
	var (
		fakeRecordSetRepo *healthinessfakes.FakeRecordSetRepo
		fakeHealthWatcher *healthinessfakes.FakeHealthWatcher
		innerRecordSet    records.RecordSet

		recordSet *healthiness.HealthyRecordSet
	)

	BeforeEach(func() {
		fakeRecordSetRepo = &healthinessfakes.FakeRecordSetRepo{}
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}

		innerRecordSet = records.RecordSet{
			Records: []records.Record{
				{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.123", Domain: "d."},
				{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.246", Domain: "d."},
			},
		}
		fakeRecordSetRepo.GetReturns(innerRecordSet, nil)
		recordSet = healthiness.NewHealthyRecordSet(fakeRecordSetRepo, fakeHealthWatcher)
	})

	It("refreshes the record set on every resolve", func() {
		for i := 0; i < 10; i++ {
			recordSet.Resolve("i.g.n.d.d.")
		}
		Expect(fakeRecordSetRepo.GetCallCount()).To(Equal(10))
	})

	It("fails when passing in a bad domain", func() {
		_, err := recordSet.Resolve("q-%%%")
		Expect(err).To(HaveOccurred())
	})

	Context("when refreshing the record set errors", func() {
		BeforeEach(func() {
			fakeRecordSetRepo.GetReturns(records.RecordSet{}, errors.New("could not fetch record set"))
		})

		It("returns error", func() {
			_, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("could not fetch record set"))
		})
	})

	Context("when all ips are healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyReturns(true)
		})

		It("returns all ips", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
		})
	})

	Context("when some ips are healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyStub = func(ip string) bool {
				switch ip {
				case "123.123.123.123":
					return true
				case "123.123.123.246":
					return false
				}
				return false
			}
		})

		It("returns only the healthy ips", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(ConsistOf("123.123.123.123"))
		})
	})

	Context("when all ips are un-healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyReturns(false)
		})

		It("returns all ips", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
		})
	})
})

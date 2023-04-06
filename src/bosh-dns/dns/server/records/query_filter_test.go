package records_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/criteria/criteriafakes"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/records"
)

var _ = Describe("QueryFilter", func() {
	var (
		qf             *records.QueryFilter
		fakeMatcher    *criteriafakes.FakeMatcher
		fakeMatchMaker *criteriafakes.FakeMatchMaker
		recs           []record.Record
		recA           record.Record
		recB           record.Record
	)

	BeforeEach(func() {
		qf = &records.QueryFilter{}
		fakeMatcher = &criteriafakes.FakeMatcher{}
		fakeMatchMaker = &criteriafakes.FakeMatchMaker{}
		recA = record.Record{
			ID: "foo",
		}
		recB = record.Record{
			ID: "bar",
		}
		recs = []record.Record{
			recA,
			recB,
		}

		fakeMatcher.MatchStub = func(r *record.Record) bool {
			switch r.ID {
			case "foo":
				return true
			case "bar":
				return false
			}
			return false
		}

		fakeMatchMaker.MatcherReturns(fakeMatcher)
	})

	It("filters records", func() {
		Expect(qf.Filter(fakeMatchMaker, recs)).To(Equal(
			[]record.Record{
				record.Record{
					ID: "foo",
				},
			},
		))
	})
})

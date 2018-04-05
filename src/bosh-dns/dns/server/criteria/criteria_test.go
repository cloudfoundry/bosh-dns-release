package criteria_test

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Criteria", func() {
	Describe("NewCriteria", func() {
		It("returns a new criteria", func() {
			c, err := criteria.NewCriteria("q-s0.q-g7.bosh", []string{"bosh"})
			Expect(err).NotTo(HaveOccurred())
			Expect(c).To(BeAssignableToTypeOf(criteria.Criteria{}))
		})

		It("returns an error when failing to parse segments", func() {
			_, err := criteria.NewCriteria("garbage", []string{})
			Expect(err).To(MatchError("domain is malformed"))
		})

		It("returns an error when failing to parse criteria", func() {
			_, err := criteria.NewCriteria("q-garbage.fire.bosh", []string{"bosh"})
			Expect(err).To(MatchError("illegal dns query"))
		})
	})

	DescribeTable("AndMatcher", func(matchings BooleanOperationMatcher) {
		matcher := new(criteria.AndMatcher)

		for _, m := range matchings.Results {
			m := m
			matcher.Append(criteria.MatcherFunc(func(_ *record.Record) bool {
				return m
			}))
		}
		Expect(matcher.Match(new(record.Record))).To(matchings)
	},
		Entry("all true", BooleanOperationMatcher{
			Expected: true,
			Results:  []bool{true, true, true, true},
		}),
		Entry("one false", BooleanOperationMatcher{
			Expected: false,
			Results:  []bool{true, false, true, true},
		}),
		Entry("one true", BooleanOperationMatcher{
			Expected: false,
			Results:  []bool{false, false, true, false},
		}),
		Entry("all false", BooleanOperationMatcher{
			Expected: false,
			Results:  []bool{false, false, false, false},
		}),
	)

	DescribeTable("OrMatcher", func(matchings BooleanOperationMatcher) {
		matcher := new(criteria.OrMatcher)

		for _, m := range matchings.Results {
			m := m
			matcher.Append(criteria.MatcherFunc(func(_ *record.Record) bool {
				return m
			}))
		}
		Expect(matcher.Match(new(record.Record))).To(matchings)
	},
		Entry("all true", BooleanOperationMatcher{
			Expected: true,
			Results:  []bool{true, true, true, true},
		}),
		Entry("one false", BooleanOperationMatcher{
			Expected: true,
			Results:  []bool{true, false, true, true},
		}),
		Entry("one true", BooleanOperationMatcher{
			Expected: true,
			Results:  []bool{false, false, true, false},
		}),
		Entry("all false", BooleanOperationMatcher{
			Expected: false,
			Results:  []bool{false, false, false, false},
		}),
	)

	Describe("FieldMatcher", func() {
		var rec *record.Record

		BeforeEach(func() {
			rec = &record.Record{
				ID:            "id",
				NumID:         "123",
				Group:         "a-group",
				Network:       "net",
				NetworkID:     "netid",
				Deployment:    "dep",
				IP:            "1.2.3.4",
				Domain:        "bosh",
				AZ:            "z1",
				AZID:          "azid",
				InstanceIndex: "0",
				GroupIDs:      []string{"gid"},
			}
		})

		DescribeTable("Matching with known fields", func(field, value string) {
			mFunc := criteria.FieldMatcher(field, value)
			Expect(mFunc(rec)).To(BeTrue())
		},
			Entry("Instance name", "instanceName", "id"),
			Entry("Instance group", "instanceGroupName", "a-group"),
			FEntry("Instance group", "instanceGroupName", "*a-group"),
			Entry("Instance group", "instanceGroupName", "a-*"),
			Entry("Instance group", "instanceGroupName", "*-group"),
			Entry("Instance group", "instanceGroupName", "*"),
			Entry("Network", "network", "net"),
			Entry("Network", "network", "net*"),
			Entry("Network", "network", "*t"),
			Entry("Network", "network", "n*"),
			Entry("Network", "network", "*"),
			Entry("Deployment name", "deployment", "dep"),
			Entry("Deployment name", "deployment", "*dep"),
			Entry("Deployment name", "deployment", "*p"),
			Entry("Deployment name", "deployment", "d*"),
			Entry("Deployment name", "deployment", "*"),
			Entry("TLD", "domain", "bosh"),
			Entry("Short-form instance", "m", "123"),
			Entry("Short-form network", "n", "netid"),
			Entry("Short-form AZ", "a", "azid"),
			Entry("Short-form index", "i", "0"),
			Entry("Group ", "g", "gid"),
		)

		DescribeTable("Matching with known fields but non-matching values", func(field, value string) {
			mFunc := criteria.FieldMatcher(field, value)
			Expect(mFunc(rec)).To(BeFalse())
		},
			Entry("Instance name", "instanceName", "id2"),
			Entry("Instance group", "instanceGroupName", "a-*roup"),
			Entry("Instance group", "instanceGroupName", "b-group"),
			Entry("Network", "network", "net2"),
			Entry("Network", "network", "n*t"),
			Entry("Deployment name", "deployment", "dep2"),
			Entry("Deployment name", "deployment", "d*p"),
			Entry("TLD", "domain", "notbosh"),
			Entry("Short-form instance", "m", "345"),
			Entry("Short-form network", "n", "netid2"),
			Entry("Short-form AZ", "a", "azid2"),
			Entry("Short-form index", "i", "1"),
			Entry("Group ", "g", "gid2"),
		)

		It("returns false when matching on an unknown field", func() {
			mFunc := criteria.FieldMatcher("bad", "no")
			Expect(mFunc(rec)).To(BeFalse())
		})
	})
})

type BooleanOperationMatcher struct {
	Expected bool
	Results  []bool
}

func (m BooleanOperationMatcher) Match(actual interface{}) (bool, error) {
	result := actual.(bool)
	if result != m.Expected {
		return false, nil
	}
	return true, nil
}

func (m BooleanOperationMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("%v should result in `%v` when put through the matcher", m.Results, m.Expected)
}

func (m BooleanOperationMatcher) NegatedFailureMessage(actual interface{}) string {
	return "not implemented"
}

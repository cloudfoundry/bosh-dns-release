package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
)

type QueryFilter struct{}

func (q *QueryFilter) Filter(crit criteria.Criteria, recs []record.Record) []record.Record {
	matcher := crit.Matcher()
	var records []record.Record

	for _, record := range recs {
		if matcher.Match(&record) {
			records = append(records, record)
		}
	}

	return records
}

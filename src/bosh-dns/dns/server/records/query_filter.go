package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
)

type QueryFilter struct{}

func (q *QueryFilter) Filter(crit criteria.Criteria, recs []record.Record) []record.Record {
	matcher := new(criteria.AndMatcher)
	for field, values := range crit {
		if field == "s" || field == "fqdn" {
			continue
		}
		matcher.Append(criteria.Field(field, values))
	}

	var records []record.Record

	for _, record := range recs {
		if matcher.Match(&record) {
			records = append(records, record)
		}
	}

	return records
}

package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
)

type QueryFilter struct{}

func (q *QueryFilter) Filter(mm criteria.MatchMaker, recs []record.Record) []record.Record {
	m := mm.Matcher()
	var records []record.Record

	for _, record := range recs {
		if m.Match(&record) {
			records = append(records, record)
		}
	}

	return records
}

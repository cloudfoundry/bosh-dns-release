package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
)

//go:generate counterfeiter . Reducer
type Reducer interface {
	Filter(criteria.MatchMaker, []record.Record) []record.Record
}

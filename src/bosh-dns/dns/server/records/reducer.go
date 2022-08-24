package records

import (
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/record"
)

//counterfeiter:generate . Reducer
type Reducer interface {
	Filter(criteria.MatchMaker, []record.Record) []record.Record
}

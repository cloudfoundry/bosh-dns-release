package records

import (
	"bosh-dns/dns/server/record"
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

type QueryEncoder struct {
	AliasDefinition

	NumID   string
	GroupID string
}

type AliasEncoder struct {
	aliases map[string][]*QueryEncoder
}

func NewAliasEncoder() *AliasEncoder {
	return &AliasEncoder{
		aliases: make(map[string][]*QueryEncoder),
	}
}

func (a *AliasEncoder) EncodeAliasesIntoQueries(rs []record.Record, as map[string][]AliasDefinition) map[string][]string {
	a.aliases = make(map[string][]*QueryEncoder)
	for domain, definitions := range as {
		domain := dns.Fqdn(domain)
		for _, definition := range definitions {
			if definition.PlaceholderType == "uuid" {
				a.AppendUUIDQueries(domain, definition, rs)
			} else {
				a.append(domain, NewQueryEncoder(definition))
			}
		}
	}

	return a.encodeDomains()
}

func (a *AliasEncoder) AppendUUIDQueries(domain string, definition AliasDefinition, rs []record.Record) {
	for _, rec := range rs {
		if rec.Domain != fmt.Sprintf("%s.", definition.RootDomain) {
			continue
		}

		found := false
		for _, groupID := range rec.GroupIDs {
			if groupID == definition.GroupID {
				found = true
				break
			}
		}

		if found {
			uuidDomain := strings.Replace(domain, "_", rec.ID, 1)
			c := NewQueryEncoder(definition)
			c.NumID = rec.NumID
			a.append(uuidDomain, c)
		}
	}
}

func (a *AliasEncoder) append(domain string, queryEncoder *QueryEncoder) {
	if _, ok := a.aliases[domain]; !ok {
		a.aliases[domain] = []*QueryEncoder{}
	}

	a.aliases[domain] = append(a.aliases[domain], queryEncoder)
}

func NewQueryEncoder(d AliasDefinition) *QueryEncoder {
	q := QueryEncoder{}
	q.HealthFilter = d.HealthFilter
	q.InitialHealthCheck = d.InitialHealthCheck
	q.GroupID = d.GroupID
	q.RootDomain = d.RootDomain
	return &q
}

// Manually kept alphabetized
func (q *QueryEncoder) encode() string {
	var sb strings.Builder
	sb.WriteString("q-")

	if q.NumID != "" {
		sb.WriteString(fmt.Sprintf("m%s", q.NumID))
	}

	switch q.HealthFilter {
	case "unhealthy":
		sb.WriteString("s1")
	case "healthy":
		sb.WriteString("s3")
	case "all":
		sb.WriteString("s4")
	default:
		sb.WriteString("s0")
	}

	switch q.InitialHealthCheck {
	case "asynchronous":
		sb.WriteString("y0")
	case "synchronous":
		sb.WriteString("y1")
	}

	sb.WriteString(fmt.Sprintf(".q-g%s.%s", q.GroupID, q.RootDomain))
	return sb.String()
}

func (a *AliasEncoder) encodeDomains() map[string][]string {
	ret := make(map[string][]string)
	for domain, queryEncoders := range a.aliases {
		if _, ok := ret[domain]; !ok {
			ret[domain] = []string{}
		}

		for _, queryEncoder := range queryEncoders {
			ret[domain] = append(ret[domain], dns.Fqdn(queryEncoder.encode()))
		}
	}
	return ret
}

package criteria

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"errors"
	"regexp"
	"strings"

	"bosh-dns/dns/server/record"
)

var keyValueRegex = regexp.MustCompile("(a|i|s|m|n|y)([0-9]+)")
var groupRegex = regexp.MustCompile("^q-g([0-9]+)$")

type Criteria map[string][]string

func NewCriteria(fqdn string, domains []string) (Criteria, error) {
	seg, err := ParseQuery(fqdn, domains)
	if seg == nil || err != nil {
		return Criteria{}, err
	}

	crit, err := parseCriteria(seg)
	if err == nil {
		crit["fqdn"] = []string{fqdn}
	}

	return crit, err
}

func (c Criteria) Matcher() Matcher {
	matcher := new(AndMatcher)
	for field, values := range c {
		if field == "y" || field == "s" || field == "fqdn" {
			continue
		}
		matcher.Append(Field(field, values))
	}

	return matcher
}

//counterfeiter:generate . MatchMaker
type MatchMaker interface {
	Matcher() Matcher
}

//counterfeiter:generate . Matcher
type Matcher interface {
	Match(r *record.Record) bool
}

type MatcherFunc func(r *record.Record) bool

func (m MatcherFunc) Match(r *record.Record) bool {
	return m(r)
}

type AndMatcher struct {
	criteria []Matcher
}

func (m *AndMatcher) Match(r *record.Record) bool {
	for _, matcher := range m.criteria {
		if !matcher.Match(r) {
			return false
		}
	}

	return true
}

func (m *AndMatcher) Append(matcher Matcher) {
	m.criteria = append(m.criteria, matcher)
}

type OrMatcher struct {
	criteria []Matcher
}

func (m *OrMatcher) Match(r *record.Record) bool {
	for _, matcher := range m.criteria {
		if matcher.Match(r) {
			return true
		}
	}

	return false
}

func (m *OrMatcher) Append(matcher Matcher) {
	m.criteria = append(m.criteria, matcher)
}

func Field(field string, values []string) Matcher {
	l := len(values)
	if l > 1 {
		or := new(OrMatcher)

		for _, value := range values {
			or.Append(FieldMatcher(field, value))
		}

		return or
	} else if l == 1 {
		return FieldMatcher(field, values[0])
	}

	return FieldMatcher("", "")
}

func globMatches(field, value string) bool {
	if value == "*" {
		return true
	} else if strings.HasPrefix(value, "*") {
		return strings.HasSuffix(field, value[1:])
	} else if strings.HasSuffix(value, "*") {
		return strings.HasPrefix(field, value[0:len(value)-1])
	}

	return false
}

func FieldMatcher(field, value string) MatcherFunc {
	switch field {

	case "instanceName":
		return func(r *record.Record) bool { return r.ID == value }
	case "instanceGroupName":
		if strings.Contains(value, "*") {
			return func(r *record.Record) bool { return globMatches(r.Group, value) }
		}
		return func(r *record.Record) bool { return r.Group == value }
	case "network":
		if strings.Contains(value, "*") {
			return func(r *record.Record) bool { return globMatches(r.Network, value) }
		}
		return func(r *record.Record) bool { return r.Network == value }
	case "deployment":
		if strings.Contains(value, "*") {
			return func(r *record.Record) bool { return globMatches(r.Deployment, value) }
		}
		return func(r *record.Record) bool { return r.Deployment == value }
	case "domain":
		return func(r *record.Record) bool { return r.Domain == value }

	case "agentID":
		return func(r *record.Record) bool { return r.AgentID == value }

	case "m":
		return func(r *record.Record) bool { return r.NumID == value }
	case "n":
		return func(r *record.Record) bool { return r.NetworkID == value }
	case "a":
		return func(r *record.Record) bool { return r.AZID == value }
	case "i":
		return func(r *record.Record) bool { return r.InstanceIndex == value }
	case "g": // array
		return func(r *record.Record) bool {
			for _, groupID := range r.GroupIDs {
				if groupID == value {
					return true
				}
			}
			return false
		}
	}

	return func(*record.Record) bool { return false }
}

func parseCriteria(qt QueryFormType) (Criteria, error) {
	criteriaMap := make(Criteria)

	switch qt.Type() {
	case SHORT:
		if err := criteriaMap.parseShortQueries(qt.Query()); err != nil {
			return nil, err
		}

		groupMatches := groupRegex.FindAllStringSubmatch(qt.(ShortForm).Group(), -1)
		if groupMatches != nil {
			criteriaMap.appendCriteria("g", groupMatches[0][1])
		}

		criteriaMap.appendCriteria("instanceName", qt.(ShortForm).Instance())
		criteriaMap.appendCriteria("domain", qt.(ShortForm).Domain())
	case LONG:

		if err := criteriaMap.parseShortQueries(qt.Query()); err != nil {
			return nil, err
		}

		criteriaMap.appendCriteria("instanceName", qt.(LongForm).Instance())
		criteriaMap.appendCriteria("instanceGroupName", qt.(LongForm).Group())
		criteriaMap.appendCriteria("network", qt.(LongForm).Network())
		criteriaMap.appendCriteria("deployment", qt.(LongForm).Deployment())

		groupMatches := groupRegex.FindAllStringSubmatch(qt.(LongForm).Group(), -1)
		if groupMatches != nil {
			criteriaMap.appendCriteria("g", groupMatches[0][1])
		}

		criteriaMap.appendCriteria("domain", qt.(LongForm).Domain())
	case AGENTID:
		criteriaMap.appendCriteria("agentID", qt.Query())
	case NONBOSH:
		criteriaMap.appendCriteria("instanceName", qt.Query())
	}

	return criteriaMap, nil
}

func isQuery(query string) bool {
	return strings.HasPrefix(query, "q-")
}

func (c Criteria) parseShortQueries(query string) error {
	if !isQuery(query) {
		return nil
	}

	query = strings.TrimPrefix(query, "q-")
	querySections := keyValueRegex.FindAllStringSubmatch(query, -1)
	if querySections == nil {
		return errors.New("illegal dns query")
	}
	for _, q := range querySections {
		c.appendCriteria(q[1], q[2])
	}
	return nil
}

func (c Criteria) appendCriteria(key, value string) {
	values, ok := c[key]
	if !ok {
		values = []string{}
	}

	if value != "" {
		c[key] = append(values, value)
	}
}

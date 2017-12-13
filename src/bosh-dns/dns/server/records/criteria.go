package records

import (
	"errors"
	"regexp"
	"strings"
)

var keyValueRegex = regexp.MustCompile("(a|i|s|m|n)([0-9]+)")
var groupRegex = regexp.MustCompile("^q-g([0-9]+)$")

type criteria map[string][]string

type Matcher interface {
	Match(r *Record) bool
}

type AndMatcher struct {
	criterion []Matcher
}

func (m *AndMatcher) Match(r *Record) bool {
	for _, matcher := range m.criterion {
		if !matcher.Match(r) {
			return false
		}
	}

	return true
}

func (m *AndMatcher) Append(matcher Matcher) {
	m.criterion = append(m.criterion, matcher)
}

type OrMatcher struct {
	criterion []Matcher
}

func (m *OrMatcher) Match(r *Record) bool {
	for _, matcher := range m.criterion {
		if matcher.Match(r) {
			return true
		}
	}

	return false
}

func (m *OrMatcher) Append(matcher Matcher) {
	m.criterion = append(m.criterion, matcher)
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

type MatcherFunc func(r *Record) bool

func (m MatcherFunc) Match(r *Record) bool {
	return m(r)
}

func FieldMatcher(field, value string) MatcherFunc {
	switch field {

	case "instanceName":
		return func(r *Record) bool { return r.ID == value }
	case "instanceGroupName":
		return func(r *Record) bool { return r.Group == value }
	case "network":
		return func(r *Record) bool { return r.Network == value }
	case "deployment":
		return func(r *Record) bool { return r.Deployment == value }
	case "domain":
		return func(r *Record) bool { return r.Domain == value }

	case "m":
		return func(r *Record) bool { return r.NumID == value }
	case "n":
		return func(r *Record) bool { return r.NetworkID == value }
	case "a":
		return func(r *Record) bool { return r.AZID == value }
	case "i":
		return func(r *Record) bool { return r.InstanceIndex == value }
	case "g": // array
		return func(r *Record) bool {
			for _, groupID := range r.GroupIDs {
				if groupID == value {
					return true
				}
			}
			return false
		}
	}

	return func(*Record) bool { return false }
}

func parseCriteria(firstSegment, groupSegment, instanceGroupName, network, deployment, domain string) (criteria, error) {
	criteriaMap := make(criteria)

	if strings.HasPrefix(firstSegment, "q-") {
		query := strings.TrimPrefix(firstSegment, "q-")
		err := criteriaMap.parseShortQueries(query)
		if err != nil {
			return nil, err
		}
	} else {
		criteriaMap.appendCriteria("instanceName", firstSegment)
	}

	groupMatches := groupRegex.FindAllStringSubmatch(groupSegment, -1)
	if groupMatches != nil {
		criteriaMap.appendCriteria("g", groupMatches[0][1])
	}
	if instanceGroupName != "" {
		criteriaMap.appendCriteria("instanceGroupName", instanceGroupName)
	}
	if network != "" {
		criteriaMap.appendCriteria("network", network)
	}
	if deployment != "" {
		criteriaMap.appendCriteria("deployment", deployment)
	}
	if domain != "" {
		criteriaMap.appendCriteria("domain", domain)
	}

	return criteriaMap, nil
}

func (c criteria) parseShortQueries(query string) error {
	querySections := keyValueRegex.FindAllStringSubmatch(query, -1)
	if querySections == nil {
		return errors.New("illegal dns query")
	}
	for _, q := range querySections {
		c.appendCriteria(q[1], q[2])
	}
	return nil
}

func (c criteria) appendCriteria(key, value string) {
	values, ok := c[key]
	if !ok {
		values = []string{}
	}

	c[key] = append(values, value)
}

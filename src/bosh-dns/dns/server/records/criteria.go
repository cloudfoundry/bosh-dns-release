package records

import (
	"errors"
	"regexp"
)

var keyValueRegex = regexp.MustCompile("(a|s)([0-9]+)")

type criteria map[string][]string

func parseCriteria(query string) (criteria, error) {
	criteriaMap := make(criteria)

	querySections := keyValueRegex.FindAllStringSubmatch(query, -1)
	if querySections == nil {
		return nil, errors.New("illegal dns query")
	}
	for _, q := range querySections {
		key := q[1]
		values, ok := criteriaMap[key]
		if !ok {
			values = []string{}
		}

		criteriaMap[key] = append(values, q[2])
	}

	return criteriaMap, nil
}

func (c criteria) isAllowed(r Record) bool {
	for key, values := range c {
		if !matchesCriterion(r, key, values) {
			return false
		}
	}
	return true
}

func matchesCriterion(r Record, key string, values []string) bool {
	var recordValue string
	switch key {
	case "a":
		recordValue = r.AZID
	case "s":
		return true
	default:
		return false
	}

	for _, v := range values {
		if recordValue == v {
			return true
		}
	}
	return false
}

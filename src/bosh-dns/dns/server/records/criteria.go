package records

import (
	"errors"
	"regexp"
	"strings"
)

var keyValueRegex = regexp.MustCompile("(a|i|s|m|n)([0-9]+)")
var groupRegex = regexp.MustCompile("^q-g([0-9]+)$")

type criteria map[string][]string

func parseCriteria(firstSegment, groupSegment, instanceGroupName, network, deployment, domain string) (criteria, error) {
	criteriaMap := make(criteria)

	if strings.HasPrefix(firstSegment, "q-") {
		query := strings.TrimPrefix(firstSegment, "q-")
		querySections := keyValueRegex.FindAllStringSubmatch(query, -1)
		if querySections == nil {
			return nil, errors.New("illegal dns query")
		}
		for _, q := range querySections {
			criteriaMap.appendCriteria(q[1], q[2])
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

func (c criteria) appendCriteria(key, value string) {
	values, ok := c[key]
	if !ok {
		values = []string{}
	}

	c[key] = append(values, value)
}

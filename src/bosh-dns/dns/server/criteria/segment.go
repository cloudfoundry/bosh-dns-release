package criteria

import (
	"errors"
	"fmt"
	"strings"
)

type Segment struct {
	Query      string
	Group      string
	Instance   string
	Network    string
	Deployment string
	Domain     string
}

func ParseSegment(fqdn string, domains []string) (Segment, error) {
	segments := strings.SplitN(fqdn, ".", 2) // [q-s0, q-g7.x.y.bosh]

	if len(segments) < 2 {
		return Segment{}, errors.New("domain is malformed")
	}

	var tld string
	for _, possible := range domains {
		if strings.HasSuffix(fqdn, possible) {
			tld = possible
			break
		}
	}

	groupQuery := strings.TrimSuffix(segments[1], "."+tld)
	groupSegments := strings.Split(groupQuery, ".")

	finalSegment := Segment{
		Query:  segments[0],
		Domain: tld,
	}

	if len(groupSegments) == 1 {
		finalSegment.Group = groupQuery
	} else if len(groupSegments) == 3 {
		finalSegment.Instance = groupSegments[0]
		finalSegment.Network = groupSegments[1]
		finalSegment.Deployment = groupSegments[2]
	} else if tld != "" {
		return Segment{}, fmt.Errorf("bad group segment query had %d values %#v", len(groupSegments), groupSegments)
	}

	return finalSegment, nil
}

package criteria

import (
	"errors"
	"fmt"
	"strings"
)

const (
	SHORT   = iota
	LONG    = iota
	AGENTID = iota
	NONBOSH = iota
)

type QueryFormType interface {
	Type() int
	Query() string
}

type ShortForm struct {
	query  string
	group  string
	domain string
}

type LongForm struct {
	ShortForm
	instance   string
	network    string
	deployment string
}

type AgentIDForm struct {
	query string
}

type NonBoshDNSForm struct {
	query string
}

func ParseQuery(fqdn string, domains []string) (QueryFormType, error) {
	segments := strings.SplitN(fqdn, ".", 2) // [q-s0, q-g7.x.y.bosh]

	if len(segments) < 2 {
		return nil, errors.New("domain is malformed")
	}

	if len(segments) == 2 && segments[1] == "" {
		return AgentIDForm{
			query: segments[0],
		}, nil
	}

	tld := findTLD(fqdn, domains)
	if tld == "" {
		return NonBoshDNSForm{query: segments[0]}, nil
	}

	groupQuery := strings.TrimSuffix(segments[1], "."+tld)
	groupSegments := strings.Split(groupQuery, ".")

	switch len(groupSegments) {
	case 1:
		return ShortForm{
			query:  segments[0],
			group:  groupQuery,
			domain: tld,
		}, nil
	case 3:
		instanceName := ""
		query := ""
		if strings.HasPrefix(segments[0], "q-") {
			query = segments[0]
		} else {
			instanceName = segments[0]
		}

		return LongForm{
			ShortForm: ShortForm{
				query:  query,
				group:  groupSegments[0],
				domain: tld,
			},
			instance:   instanceName,
			network:    groupSegments[1],
			deployment: groupSegments[2],
		}, nil
	}

	if tld != "" {
		return nil, fmt.Errorf("bad group segment query had %d values %#v", len(groupSegments), groupSegments)
	}

	// ShortForm ends up being the default.
	return ShortForm{query: segments[0], domain: tld}, nil
}

func findTLD(fqdn string, domains []string) string {
	for _, possible := range domains {
		if strings.HasSuffix(fqdn, possible) {
			return possible
		}
	}

	return ""
}

func (s ShortForm) Type() int {
	return SHORT
}

func (s ShortForm) Query() string {
	return s.query
}

func (s ShortForm) Group() string {
	return s.group
}

func (s ShortForm) Domain() string {
	return s.domain
}

func (s ShortForm) Deployment() string {
	return ""
}

func (s LongForm) Type() int {
	return LONG
}

func (s LongForm) Query() string {
	return s.query
}

func (s LongForm) Group() string {
	return s.group
}

func (s LongForm) Domain() string {
	return s.domain
}

func (s LongForm) Instance() string {
	return s.instance
}

func (s LongForm) Network() string {
	return s.network
}

func (s LongForm) Deployment() string {
	return s.deployment
}

func (s AgentIDForm) Type() int {
	return AGENTID
}

func (s AgentIDForm) Query() string {
	return s.query
}

func (s NonBoshDNSForm) Type() int {
	return NONBOSH
}

func (s NonBoshDNSForm) Query() string {
	return s.query
}

func NewShortFormQuery(query, group, domain string) ShortForm {
	return ShortForm{
		query:  query,
		group:  group,
		domain: domain,
	}
}

func NewLongFormQuery(query, group, domain, instance, network, deployment string) LongForm {
	return LongForm{
		ShortForm:  NewShortFormQuery(query, group, domain),
		instance:   instance,
		network:    network,
		deployment: deployment,
	}
}

func NewAgentIDFormQuery(query string) AgentIDForm {
	return AgentIDForm{
		query: query,
	}
}

func NewNonBoshDNSQuery(query string) NonBoshDNSForm {
	return NonBoshDNSForm{
		query: query,
	}
}

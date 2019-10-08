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

const BoshAgentTLD = "bosh-agent-id"

type QueryFormType interface {
	Type() int
	Query() string
}

type ShortForm struct {
	query    string
	group    string
	domain   string
	instance string
}

type LongForm struct {
	ShortForm
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

	if len(segments) == 2 && segments[1] == fmt.Sprintf("%s.", BoshAgentTLD) {
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
	instanceName := ""
	query := ""
	if isQuery(segments[0]) {
		query = segments[0]
	} else {
		instanceName = segments[0]
	}

	switch len(groupSegments) {
	case 1:
		return ShortForm{
			query:    query,
			instance: instanceName,
			group:    groupQuery,
			domain:   tld,
		}, nil
	case 3:
		return LongForm{
			ShortForm: ShortForm{
				query:    query,
				group:    groupSegments[0],
				domain:   tld,
				instance: instanceName,
			},
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

func (s ShortForm) Instance() string {
	return s.instance
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

func NewShortFormQuery(query, instance, group, domain string) ShortForm {
	return ShortForm{
		query:    query,
		instance: instance,
		group:    group,
		domain:   domain,
	}
}

func NewLongFormQuery(query, group, domain, instance, network, deployment string) LongForm {
	return LongForm{
		ShortForm:  NewShortFormQuery(query, instance, group, domain),
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

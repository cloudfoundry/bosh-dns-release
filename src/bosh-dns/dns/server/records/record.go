package records

import "strings"

type Record struct {
	ID         string
	Group      string
	Network    string
	Deployment string
	IP         string
	Domain     string
	AZID       string
}

func (r Record) Fqdn(includeJobLabel bool) string {
	var fields []string

	if includeJobLabel {
		fields = []string{r.ID, r.Group, r.Network, r.Deployment, r.Domain}
	} else {
		fields = []string{r.Group, r.Network, r.Deployment, r.Domain}
	}

	return strings.Join(fields, ".")
}

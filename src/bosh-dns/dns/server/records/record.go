package records

import "strings"

type Record struct {
	Id         string
	Group      string
	Network    string
	Deployment string
	Ip         string
	Domain     string
	AzId    string
}

func (r Record) Fqdn(includeJobLabel bool) string {
	var fields []string

	if includeJobLabel {
		fields = []string{r.Id, r.Group, r.Network, r.Deployment, r.Domain}
	} else {
		fields = []string{r.Group, r.Network, r.Deployment, r.Domain}
	}

	return strings.Join(fields, ".")
}

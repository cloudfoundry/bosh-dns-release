package records

import (
	"encoding/base64"
	"strings"
)

type RecordSet struct {
	Keys            []string   `json:"record_keys"`
	Infos           [][]string `json:"record_infos"`
	idIndex         int
	groupIndex      int
	networkIndex    int
	deploymentIndex int
	ipIndex         int
}

func (r RecordSet) Resolve(fqdn string) ([]string, error) {
	r.calculateIndicies()

	var ips []string

	if strings.HasPrefix(fqdn, "q-") {
		matcher := strings.SplitN(fqdn, ".", 2)
		base64EncodedQuery := strings.TrimPrefix(matcher[0], "q-")
		decodedQuery, err := base64.RawURLEncoding.DecodeString(base64EncodedQuery)
		if err != nil {
			return ips, err
		}

		if string(decodedQuery) == "all" {
			for _, i := range r.Infos {
				record := strings.Join([]string{i[r.groupIndex], i[r.networkIndex], i[r.deploymentIndex], "bosh."}, ".")
				if record == matcher[1] {
					ips = append(ips, i[r.ipIndex])
				}
			}
		}
	} else {
		for _, i := range r.Infos {
			compare := strings.Join([]string{i[r.idIndex], i[r.groupIndex], i[r.networkIndex], i[r.deploymentIndex], "bosh."}, ".")
			if compare == fqdn {
				ips = append(ips, i[r.ipIndex])
			}
		}
	}

	return ips, nil
}

func (r *RecordSet) calculateIndicies() {
	for i, k := range r.Keys {
		switch k {
		case "id":
			r.idIndex = i
		case "instance_group":
			r.groupIndex = i
		case "network":
			r.networkIndex = i
		case "deployment":
			r.deploymentIndex = i
		case "ip":
			r.ipIndex = i
		default:
			continue
		}
	}
}

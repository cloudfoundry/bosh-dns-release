package records

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type RecordSet struct {
	Records []Record
}

func (r RecordSet) Resolve(fqdn string) ([]string, error) {
	var ips []string

	if strings.HasPrefix(fqdn, "q-") {
		matcher := strings.SplitN(fqdn, ".", 2)
		base64EncodedQuery := strings.TrimPrefix(matcher[0], "q-")
		decodedQuery, err := base64.RawURLEncoding.DecodeString(base64EncodedQuery)
		if err != nil {
			return ips, err
		}

		if string(decodedQuery) == "all" {
			for _, record := range r.Records {
				recordName := record.Fqdn(false)
				if recordName == matcher[1] {
					ips = append(ips, record.Ip)
				}
			}
		}
	} else {
		for _, record := range r.Records {
			compare := record.Fqdn(true)
			if compare == fqdn {
				ips = append(ips, record.Ip)
			}
		}
	}

	return ips, nil
}

func (s *RecordSet) UnmarshalJSON(j []byte) error {
	swap := struct {
		Keys  []string   `json:"record_keys"`
		Infos [][]string `json:"record_infos"`
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		return err
	}

	s.Records = make([]Record, len(swap.Infos))

	var idIndex,
	groupIndex,
	networkIndex,
	deploymentIndex,
	ipIndex int

	for i, k := range swap.Keys {
		switch k {
		case "id":
			idIndex = i
		case "instance_group":
			groupIndex = i
		case "network":
			networkIndex = i
		case "deployment":
			deploymentIndex = i
		case "ip":
			ipIndex = i
		default:
			continue
		}
	}

	for index, info := range swap.Infos {
		s.Records[index] = Record{
			Id:         info[idIndex],
			Group:      info[groupIndex],
			Network:    info[networkIndex],
			Deployment: info[deploymentIndex],
			Ip:         info[ipIndex],
		}
	}

	return nil
}

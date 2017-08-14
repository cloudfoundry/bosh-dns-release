package records

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"errors"
	"github.com/miekg/dns"
)

type RecordSet struct {
	Domains []string
	Records []Record
}

func (r RecordSet) Resolve(fqdn string) ([]string, error) {
	if net.ParseIP(fqdn) != nil {
		return []string{fqdn}, nil
	}

	var ips []string

	if strings.HasPrefix(fqdn, "q-") {
		matcher := strings.SplitN(fqdn, ".", 2)
		if len(matcher) < 2 {
			return ips, errors.New("domain is malformed")
		}
		encodedQuery := strings.TrimPrefix(matcher[0], "q-")
		filter, err := parseCriteria(encodedQuery)
		if err != nil {
			return ips, err
		}

		for _, record := range r.Records {
			recordName := record.Fqdn(false)
			if recordName == matcher[1] && filter.isAllowed(record) {
				ips = append(ips, record.Ip)
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
		Keys  []string        `json:"record_keys"`
		Infos [][]interface{} `json:"record_infos"`
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		return err
	}

	s.Records = make([]Record, len(swap.Infos))
	s.Domains = []string{}

	var idIndex,
		groupIndex,
		networkIndex,
		deploymentIndex,
		ipIndex,
		azIdIndex,
		domainIndex int

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
		case "domain":
			domainIndex = i
		case "az_id":
			azIdIndex = i
		default:
			continue
		}
	}

	domains := map[string]struct{}{}
	countKeys := len(swap.Keys)

	for index, info := range swap.Infos {
		countInfo := len(info)
		if countInfo != countKeys {
			return fmt.Errorf("Unbalanced records structure. Found %d fields of an expected %d at record #%d", countInfo, countKeys, index)
		}

		domain := dns.Fqdn(info[domainIndex].(string))
		domains[domain] = struct{}{}

		s.Records[index] = Record{
			Id:         info[idIndex].(string),
			Group:      info[groupIndex].(string),
			Network:    info[networkIndex].(string),
			Deployment: info[deploymentIndex].(string),
			Ip:         info[ipIndex].(string),
			AzIndex:    info[azIdIndex].(string),
			Domain:     domain,
		}
	}

	for domain := range domains {
		s.Domains = append(s.Domains, domain)
	}

	return nil
}

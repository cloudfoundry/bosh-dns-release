package records

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"

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
	s.Domains = []string{}

	var idIndex,
		groupIndex,
		networkIndex,
		deploymentIndex,
		ipIndex,
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

		domain := dns.Fqdn(info[domainIndex])
		domains[domain] = struct{}{}

		s.Records[index] = Record{
			Id:         info[idIndex],
			Group:      info[groupIndex],
			Network:    info[networkIndex],
			Deployment: info[deploymentIndex],
			Ip:         info[ipIndex],
			Domain:     domain,
		}
	}

	for domain := range domains {
		s.Domains = append(s.Domains, domain)
	}

	return nil
}

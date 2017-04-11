package records

import (
	"encoding/json"
	"io/ioutil"
	"strings"
)

type Repo struct {
	recordsFilePath string
}

type records struct {
	Keys  []string   `json:"record_keys"`
	Infos [][]string `json:"record_infos"`
}

func NewRepo(recordsFilePath string) Repo {
	return Repo{
		recordsFilePath: recordsFilePath,
	}
}

func (r Repo) GetIPs(fqdn string) ([]string, error) {
	buf, err := ioutil.ReadFile(r.recordsFilePath)
	if err != nil {
		return []string{}, err
	}

	var records records
	if err := json.Unmarshal(buf, &records); err != nil {
		return []string{}, err
	}

	var idIndex, groupIndex, networkIndex, deploymentIndex, ipIndex int

	for i, k := range records.Keys {
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
	var ips []string

	for _, i := range records.Infos {
		compare := strings.Join([]string{i[idIndex], i[groupIndex], i[networkIndex], i[deploymentIndex], "bosh."}, ".")
		if compare == fqdn {
			ips = append(ips, i[ipIndex])
		}
	}

	return ips, nil
}

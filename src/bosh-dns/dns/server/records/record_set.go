package records

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"errors"

	"strconv"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type recordGroup map[*Record]struct{}

type RecordSet struct {
	Domains         []string
	Records         []*Record
	byNumId         map[string]recordGroup
	byNetworkID     map[string]recordGroup
	byAzId          map[string]recordGroup
	byInstanceIndex map[string]recordGroup
	byInstanceGroup map[string]recordGroup
	byNetwork       map[string]recordGroup
	byDeployment    map[string]recordGroup
	byGroupID       map[string]recordGroup
	byInstanceName  map[string]recordGroup
	byDomain        map[string]recordGroup
}

func NewRecordSet(records []*Record) RecordSet {
	r := RecordSet{}
	r.byNumId = make(map[string]recordGroup)
	r.byNetworkID = make(map[string]recordGroup)
	r.byAzId = make(map[string]recordGroup)
	r.byInstanceIndex = make(map[string]recordGroup)
	r.byGroupID = make(map[string]recordGroup)
	r.byInstanceGroup = make(map[string]recordGroup)
	r.byNetwork = make(map[string]recordGroup)
	r.byDeployment = make(map[string]recordGroup)
	r.byInstanceName = make(map[string]recordGroup)
	r.byDomain = make(map[string]recordGroup)
	r.Records = records

	domains := make(map[string]struct{})
	for _, record := range r.Records {
		if r.byNumId[record.NumId] == nil {
			r.byNumId[record.NumId] = make(recordGroup)
		}
		r.byNumId[record.NumId][record] = struct{}{}

		if r.byNetworkID[record.NetworkID] == nil {
			r.byNetworkID[record.NetworkID] = make(recordGroup)
		}
		r.byNetworkID[record.NetworkID][record] = struct{}{}

		if r.byAzId[record.AZID] == nil {
			r.byAzId[record.AZID] = make(recordGroup)
		}
		if r.byInstanceIndex[record.InstanceIndex] == nil {
			r.byInstanceIndex[record.InstanceIndex] = make(recordGroup)
		}

		r.byAzId[record.AZID][record] = struct{}{}
		r.byInstanceIndex[record.InstanceIndex][record] = struct{}{}

		for _, groupID := range record.GroupIDs {
			if r.byGroupID[groupID] == nil {
				r.byGroupID[groupID] = make(recordGroup)
			}
			r.byGroupID[groupID][record] = struct{}{}
		}

		if r.byInstanceGroup[record.Group] == nil {
			r.byInstanceGroup[record.Group] = make(recordGroup)
		}
		r.byInstanceGroup[record.Group][record] = struct{}{}

		if r.byNetwork[record.Network] == nil {
			r.byNetwork[record.Network] = make(recordGroup)
		}
		r.byNetwork[record.Network][record] = struct{}{}

		if r.byDeployment[record.Deployment] == nil {
			r.byDeployment[record.Deployment] = make(recordGroup)
		}
		r.byDeployment[record.Deployment][record] = struct{}{}

		if r.byInstanceName[record.ID] == nil {
			r.byInstanceName[record.ID] = make(recordGroup)
		}
		r.byInstanceName[record.ID][record] = struct{}{}
		domains[record.Domain] = struct{}{}

		if r.byDomain[record.Domain] == nil {
			r.byDomain[record.Domain] = make(recordGroup)
		}
		r.byDomain[record.Domain][record] = struct{}{}

		domains[record.Domain] = struct{}{}
	}
	for domain := range domains {
		r.Domains = append(r.Domains, domain)
	}

	return r
}

func (r RecordSet) Resolve(fqdn string) ([]string, error) {
	if net.ParseIP(fqdn) != nil {
		return []string{fqdn}, nil
	}

	return r.resolveQuery(fqdn)
}

func (r recordGroup) union(anotherGroup recordGroup) recordGroup {
	unionedGroup := make(recordGroup)
	for record := range r {
		unionedGroup[record] = struct{}{}
	}
	for record := range anotherGroup {
		unionedGroup[record] = struct{}{}
	}
	return unionedGroup
}

func (r recordGroup) intersect(anotherGroup recordGroup) recordGroup {
	intersectedGroup := make(recordGroup)
	for record := range r {
		if _, ok := anotherGroup[record]; ok {
			intersectedGroup[record] = struct{}{}
		}
	}
	return intersectedGroup
}

// (recordGroup recordSet filter) -> recordGroup
func (r RecordSet) allRecords() recordGroup {
	records := make(recordGroup)
	for i, _ := range r.Records {
		records[r.Records[i]] = struct{}{}
	}

	return records
}

func (r RecordSet) recordsFrom(sources []string, groupFunc func(RecordSet) map[string]recordGroup) recordGroup {
	if len(sources) == 0 {
		return r.allRecords()
	}

	unionedRecords := make(recordGroup)
	for _, source := range sources {
		unionedRecords = unionedRecords.union(groupFunc(r)[source])
	}
	return unionedRecords
}

func (r RecordSet) recordsFromGroup(groupIDs []string) recordGroup {
	return r.recordsFrom(groupIDs, func(rs RecordSet) map[string]recordGroup { return r.byGroupID })
}

func (r RecordSet) recordsFromInstanceIndices(idxs []string) recordGroup {
	return r.recordsFrom(idxs, func(rs RecordSet) map[string]recordGroup { return r.byInstanceIndex })
}

func (r RecordSet) recordsFromAZs(azs []string) recordGroup {
	return r.recordsFrom(azs, func(rs RecordSet) map[string]recordGroup { return r.byAzId })
}

func (r RecordSet) recordsFromNumId(numIds []string) recordGroup {
	return r.recordsFrom(numIds, func(rs RecordSet) map[string]recordGroup { return r.byNumId })
}

func (r RecordSet) recordsFromNetworkID(networkIDs []string) recordGroup {
	return r.recordsFrom(networkIDs, func(rs RecordSet) map[string]recordGroup { return r.byNetworkID })
}

func (r RecordSet) recordsFromNetwork(networks []string) recordGroup {
	return r.recordsFrom(networks, func(rs RecordSet) map[string]recordGroup { return r.byNetwork })
}

func (r RecordSet) recordsFromInstanceGroupName(instanceGroupNames []string) recordGroup {
	return r.recordsFrom(instanceGroupNames, func(rs RecordSet) map[string]recordGroup { return r.byInstanceGroup })
}

func (r RecordSet) recordsFromDeployment(deployments []string) recordGroup {
	return r.recordsFrom(deployments, func(rs RecordSet) map[string]recordGroup { return r.byDeployment })
}

func (r RecordSet) recordsFromInstanceName(instanceNames []string) recordGroup {
	return r.recordsFrom(instanceNames, func(rs RecordSet) map[string]recordGroup { return r.byInstanceName })
}

func (r RecordSet) recordsFromDomain(domains []string) recordGroup {
	return r.recordsFrom(domains, func(rs RecordSet) map[string]recordGroup { return r.byDomain })
}

func (r RecordSet) resolveQuery(fqdn string) ([]string, error) {
	var ips []string

	segments := strings.SplitN(fqdn, ".", 2) // [q-s0, q-g7.x.y.bosh]

	if len(segments) < 2 {
		return ips, errors.New("domain is malformed")
	}

	var tld string
	for _, possible := range r.Domains { // do these/do these have to end in a . ?
		if strings.HasSuffix(fqdn, possible) {
			tld = possible
			break
		}
	}

	if tld == "" {
		return []string{}, nil
	}

	groupQuery := strings.TrimSuffix(segments[1], "."+tld)
	groupSegments := strings.Split(groupQuery, ".")
	var filter criteria
	var err error
	if len(groupSegments) == 1 {
		filter, err = parseCriteria(segments[0], groupQuery, "", "", "", tld)
		if err != nil {
			return ips, err
		}
	} else if len(groupSegments) == 3 {
		filter, err = parseCriteria(segments[0], "", groupSegments[0], groupSegments[1], groupSegments[2], tld)
		if err != nil {
			return ips, err
		}
	} else {
		panic(fmt.Sprintf("Bad group segment query had %d values %#v\n", len(groupSegments), groupSegments))
	}

	allRecords := r.allRecords()

	candidates := allRecords
	candidates = candidates.intersect(r.recordsFromNumId(filter["m"]))
	candidates = candidates.intersect(r.recordsFromNetworkID(filter["n"]))
	candidates = candidates.intersect(r.recordsFromAZs(filter["a"]))
	candidates = candidates.intersect(r.recordsFromInstanceIndices(filter["i"]))
	candidates = candidates.intersect(r.recordsFromGroup(filter["g"]))
	candidates = candidates.intersect(r.recordsFromNetwork(filter["network"]))
	candidates = candidates.intersect(r.recordsFromInstanceGroupName(filter["instanceGroupName"]))
	candidates = candidates.intersect(r.recordsFromDeployment(filter["deployment"]))
	candidates = candidates.intersect(r.recordsFromInstanceName(filter["instanceName"]))
	candidates = candidates.intersect(r.recordsFromDomain(filter["domain"]))

	for record := range candidates {
		ips = append(ips, (*record).IP)
	}
	return ips, nil
}

func CreateFromJSON(j []byte, logger boshlog.Logger) (RecordSet, error) {
	swap := struct {
		Keys  []string        `json:"record_keys"`
		Infos [][]interface{} `json:"record_infos"`
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		return RecordSet{}, err
	}

	records := make([]*Record, 0, len(swap.Infos))

	idIndex := -1
	numIdIndex := -1
	groupIndex := -1
	networkIndex := -1
	networkIDIndex := -1
	deploymentIndex := -1
	ipIndex := -1
	domainIndex := -1
	azIDIndex := -1
	instanceIndexIndex := -1
	groupIdsIndex := -1

	for i, k := range swap.Keys {
		switch k {
		case "id":
			idIndex = i
		case "num_id":
			numIdIndex = i
		case "instance_group":
			groupIndex = i
		case "group_ids":
			groupIdsIndex = i
		case "network":
			networkIndex = i
		case "network_id":
			networkIDIndex = i
		case "deployment":
			deploymentIndex = i
		case "ip":
			ipIndex = i
		case "domain":
			domainIndex = i
		case "az_id":
			azIDIndex = i
		case "instance_index":
			instanceIndexIndex = i
		default:
			continue
		}
	}

	countKeys := len(swap.Keys)

	for index, info := range swap.Infos {
		countInfo := len(info)
		if countInfo != countKeys {
			logger.Warn("RecordSet", "Unbalanced records structure. Found %d fields of an expected %d at record #%d", countInfo, countKeys, index)
			continue
		}

		var domainIndexStr string
		if !requiredStringValue(&domainIndexStr, info, domainIndex, "domain", index, logger) {
			continue
		}

		domain := dns.Fqdn(domainIndexStr)

		record := Record{Domain: domain}

		if !requiredStringValue(&record.ID, info, idIndex, "id", index, logger) {
			continue
		} else if !requiredStringValue(&record.Group, info, groupIndex, "group", index, logger) {
			continue
		} else if !requiredStringValue(&record.Network, info, networkIndex, "network", index, logger) {
			continue
		} else if !requiredStringValue(&record.Deployment, info, deploymentIndex, "deployment", index, logger) {
			continue
		} else if !requiredStringValue(&record.IP, info, ipIndex, "ip", index, logger) {
			continue
		} else if !optionalStringValue(&record.AZID, info, azIDIndex, "az_id", index, logger) {
			continue
		} else if !optionalStringValue(&record.NetworkID, info, networkIDIndex, "network_id", index, logger) {
			continue
		} else if !optionalStringValue(&record.NumId, info, numIdIndex, "num_id", index, logger) {
			continue
		} else if groupIdsIndex >= 0 && !assertStringArrayOfStringValue(&record.GroupIDs, info, groupIdsIndex, "group_ids", index, logger) {
			continue
		}

		assertStringIntegerValue(&record.InstanceIndex, info, instanceIndexIndex, "instance_index", index, logger)

		records = append(records, &record)
	}

	return NewRecordSet(records), nil
}

func assertStringIntegerValue(field *string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	if fieldIdx < 0 {
		return false
	}

	float64Value, ok := info[fieldIdx].(float64) // golang default type for numeric fields
	if !ok {
		logger.Warn("RecordSet", "Value %d (%s) of record %d is not expected type of %s: %#+v", fieldIdx, fieldName, infoIdx, "numeric", info[fieldIdx])
	}

	*field = strconv.Itoa(int(float64Value))
	return ok
}

func convertToStringValue(field *string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	var ok bool
	*field, ok = info[fieldIdx].(string)

	if !ok {
		logger.Warn("RecordSet", "Value %d (%s) of record %d is not expected type of %s: %#+v", fieldIdx, fieldName, infoIdx, "string", info[fieldIdx])
	}

	return ok
}

func optionalStringValue(field *string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	if fieldIdx >= 0 {
		if info[fieldIdx] == nil {
			info[fieldIdx] = ""
			return true
		}
		return convertToStringValue(field, info, fieldIdx, fieldName, infoIdx, logger)
	}

	return true
}

func requiredStringValue(field *string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	if fieldIdx < 0 {
		return false
	}

	return convertToStringValue(field, info, fieldIdx, fieldName, infoIdx, logger)
}

func assertStringArrayOfStringValue(field *[]string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	var ok bool
	var intermediateField []interface{}

	intermediateField, ok = info[fieldIdx].([]interface{})
	if !ok {
		logger.Warn("RecordSet", "Value %d (%s) of record %d is not expected type of %s: %#+v", fieldIdx, fieldName, infoIdx, "array of string", info[fieldIdx])
	}
	out := make([]string, len(intermediateField))
	for i, v := range intermediateField {
		out[i], ok = v.(string)
		if !ok {
			logger.Warn("RecordSet", "Value %d (%s) of record %d is not expected type of %s: %#+v", fieldIdx, fieldName, infoIdx, "array of string", info[fieldIdx])
			return ok
		}
	}

	*field = out

	return ok
}

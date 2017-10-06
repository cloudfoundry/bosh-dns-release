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
	byGlobalIndex   map[string]recordGroup
	byNetworkID     map[string]recordGroup
	ByAzId          map[string]recordGroup
	ByInstanceIndex map[string]recordGroup
	ByInstanceGroup map[string]recordGroup
	ByNetwork       map[string]recordGroup
	ByDeployment    map[string]recordGroup
	ByGroupID       map[string]recordGroup
	ByInstanceName  map[string]recordGroup
	ByDomain        map[string]recordGroup
	initialized     bool
}

func NewRecordSet(records []*Record) RecordSet {
	r := RecordSet{}
	r.byGlobalIndex = make(map[string]recordGroup)
	r.byNetworkID = make(map[string]recordGroup)
	r.ByAzId = make(map[string]recordGroup)
	r.ByInstanceIndex = make(map[string]recordGroup)
	r.ByGroupID = make(map[string]recordGroup)
	r.ByInstanceGroup = make(map[string]recordGroup)
	r.ByNetwork = make(map[string]recordGroup)
	r.ByDeployment = make(map[string]recordGroup)
	r.ByInstanceName = make(map[string]recordGroup)
	r.ByDomain = make(map[string]recordGroup)
	r.Records = records

	domains := make(map[string]struct{})
	for _, record := range r.Records {
		if r.byGlobalIndex[record.GlobalIndex] == nil {
			r.byGlobalIndex[record.GlobalIndex] = make(recordGroup)
		}
		r.byGlobalIndex[record.GlobalIndex][record] = struct{}{}

		if r.byNetworkID[record.NetworkID] == nil {
			r.byNetworkID[record.NetworkID] = make(recordGroup)
		}
		r.byNetworkID[record.NetworkID][record] = struct{}{}

		if r.ByAzId[record.AZID] == nil {
			r.ByAzId[record.AZID] = make(recordGroup)
		}
		if r.ByInstanceIndex[record.InstanceIndex] == nil {
			r.ByInstanceIndex[record.InstanceIndex] = make(recordGroup)
		}

		r.ByAzId[record.AZID][record] = struct{}{}
		r.ByInstanceIndex[record.InstanceIndex][record] = struct{}{}

		for _, groupID := range record.GroupIDs {
			if r.ByGroupID[groupID] == nil {
				r.ByGroupID[groupID] = make(recordGroup)
			}
			r.ByGroupID[groupID][record] = struct{}{}
		}

		if r.ByInstanceGroup[record.Group] == nil {
			r.ByInstanceGroup[record.Group] = make(recordGroup)
		}
		r.ByInstanceGroup[record.Group][record] = struct{}{}

		if r.ByNetwork[record.Network] == nil {
			r.ByNetwork[record.Network] = make(recordGroup)
		}
		r.ByNetwork[record.Network][record] = struct{}{}

		if r.ByDeployment[record.Deployment] == nil {
			r.ByDeployment[record.Deployment] = make(recordGroup)
		}
		r.ByDeployment[record.Deployment][record] = struct{}{}

		if r.ByInstanceName[record.ID] == nil {
			r.ByInstanceName[record.ID] = make(recordGroup)
		}
		r.ByInstanceName[record.ID][record] = struct{}{}
		domains[record.Domain] = struct{}{}

		if r.ByDomain[record.Domain] == nil {
			r.ByDomain[record.Domain] = make(recordGroup)
		}
		r.ByDomain[record.Domain][record] = struct{}{}

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

func (r RecordSet) recordFromGroup(groupIDs []string) recordGroup {
	if len(groupIDs) == 0 {
		return r.allRecords()
	}

	unionedGroups := make(recordGroup)
	for _, groupID := range groupIDs {
		unionedGroups = unionedGroups.union(r.ByGroupID[groupID])
	}
	return unionedGroups
}

func (r RecordSet) recordFromInstanceIndices(idxs []string) recordGroup {
	if len(idxs) == 0 {
		return r.allRecords()
	}

	unionedIdxs := make(recordGroup)
	for _, idx := range idxs {
		unionedIdxs = unionedIdxs.union(r.ByInstanceIndex[idx])
	}
	return unionedIdxs
}

func (r RecordSet) recordFromAZs(azs []string) recordGroup {
	if len(azs) == 0 {
		return r.allRecords()
	}

	unionedAzs := make(recordGroup)
	for _, az := range azs {
		unionedAzs = unionedAzs.union(r.ByAzId[az])
	}
	return unionedAzs
}

func (r RecordSet) recordFromGlobalIndex(globalIndexes []string) recordGroup {
	if len(globalIndexes) == 0 {
		return r.allRecords()
	}

	unionedGlobalIndexes := make(recordGroup)
	for _, globalIndex := range globalIndexes {
		unionedGlobalIndexes = unionedGlobalIndexes.union(r.byGlobalIndex[globalIndex])
	}
	return unionedGlobalIndexes
}

func (r RecordSet) recordFromNetworkID(networkIDs []string) recordGroup {
	if len(networkIDs) == 0 {
		return r.allRecords()
	}

	unionedNetworkIDs := make(recordGroup)
	for _, globalIndex := range networkIDs {
		unionedNetworkIDs = unionedNetworkIDs.union(r.byNetworkID[globalIndex])
	}
	return unionedNetworkIDs
}

func (r RecordSet) recordFromNetwork(networks []string) recordGroup {
	if len(networks) == 0 {
		return r.allRecords()
	}

	unionedNetworks := make(recordGroup)
	for _, network := range networks {
		unionedNetworks = unionedNetworks.union(r.ByNetwork[network])
	}
	return unionedNetworks
}

func (r RecordSet) recordFromInstanceGroupName(instanceGroupNames []string) recordGroup {
	if len(instanceGroupNames) == 0 {
		return r.allRecords()
	}

	unionedIgNames := make(recordGroup)
	for _, names := range instanceGroupNames {
		unionedIgNames = unionedIgNames.union(r.ByInstanceGroup[names])
	}
	return unionedIgNames
}

func (r RecordSet) recordFromDeployment(deployments []string) recordGroup {
	if len(deployments) == 0 {
		return r.allRecords()
	}

	unionedDeployments := make(recordGroup)
	for _, deployment := range deployments {
		unionedDeployments = unionedDeployments.union(r.ByDeployment[deployment])
	}
	return unionedDeployments
}

func (r RecordSet) recordFromInstanceName(instanceNames []string) recordGroup {
	if len(instanceNames) == 0 {
		return r.allRecords()
	}

	unionedInstanceNames := make(recordGroup)
	for _, instanceName := range instanceNames {
		instances := []Record{}
		for r, _ := range r.ByInstanceName[instanceName] {
			instances = append(instances, *r)
		}
		unionedInstanceNames = unionedInstanceNames.union(r.ByInstanceName[instanceName])
	}
	return unionedInstanceNames
}

func (r RecordSet) recordFromDomain(domains []string) recordGroup {
	if len(domains) == 0 {
		return r.allRecords()
	}

	unionedDomains := make(recordGroup)
	for _, domain := range domains {
		instances := []Record{}
		for r, _ := range r.ByDomain[domain] {
			instances = append(instances, *r)
		}
		unionedDomains = unionedDomains.union(r.ByDomain[domain])
	}
	return unionedDomains
}

func (r RecordSet) resolveQuery(fqdn string) ([]string, error) {
	var ips []string

	segments := strings.SplitN(fqdn, ".", 2) // [q-s0, g-7.x.y.bosh]

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
	candidates = candidates.intersect(r.recordFromGlobalIndex(filter["m"]))
	candidates = candidates.intersect(r.recordFromNetworkID(filter["n"]))
	candidates = candidates.intersect(r.recordFromAZs(filter["a"]))
	candidates = candidates.intersect(r.recordFromInstanceIndices(filter["i"]))
	candidates = candidates.intersect(r.recordFromGroup(filter["g"]))
	candidates = candidates.intersect(r.recordFromNetwork(filter["network"]))
	candidates = candidates.intersect(r.recordFromInstanceGroupName(filter["instanceGroupName"]))
	candidates = candidates.intersect(r.recordFromDeployment(filter["deployment"]))
	candidates = candidates.intersect(r.recordFromInstanceName(filter["instanceName"]))
	candidates = candidates.intersect(r.recordFromDomain(filter["domain"]))

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
	globalIndexIndex := -1
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
		case "global_index":
			globalIndexIndex = i
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
		} else if !optionalStringValue(&record.GlobalIndex, info, globalIndexIndex, "global_index", index, logger) {
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

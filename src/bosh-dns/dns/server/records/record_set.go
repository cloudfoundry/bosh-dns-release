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

func (r *RecordSet) loadUp() {
	if !r.initialized {
		r.ByAzId = make(map[string]recordGroup)
		r.ByInstanceIndex = make(map[string]recordGroup)
		r.ByGroupID = make(map[string]recordGroup)
		r.ByInstanceGroup = make(map[string]recordGroup)
		r.ByNetwork = make(map[string]recordGroup)
		r.ByDeployment = make(map[string]recordGroup)
		r.ByInstanceName = make(map[string]recordGroup)
		r.ByDomain = make(map[string]recordGroup)

		domains := make(map[string]struct{})
		for _, record := range r.Records {
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
	}
	r.initialized = true
}

func (r RecordSet) Resolve(fqdn string) ([]string, error) {
	if net.ParseIP(fqdn) != nil {
		return []string{fqdn}, nil
	}
	r.loadUp()

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
		return []string{}, nil //fmt.Errorf(fmt.Sprintf("no possible TLDs for %#v; possible domains were %#v", fqdn, r.Domains))
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
	s := RecordSet{}
	swap := struct {
		Keys  []string        `json:"record_keys"`
		Infos [][]interface{} `json:"record_infos"`
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		return RecordSet{}, err
	}

	s.Records = make([]*Record, 0, len(swap.Infos))
	s.Domains = []string{}

	idIndex := -1
	groupIndex := -1
	networkIndex := -1
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
		case "instance_group":
			groupIndex = i
		case "group_ids":
			groupIdsIndex = i
		case "network":
			networkIndex = i
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
		if !assertStringValue(&domainIndexStr, info, domainIndex, "domain", index, logger) {
			continue
		}

		domain := dns.Fqdn(domainIndexStr)

		record := Record{Domain: domain}

		if !assertStringValue(&record.ID, info, idIndex, "id", index, logger) {
			continue
		} else if !assertStringValue(&record.Group, info, groupIndex, "group", index, logger) {
			continue
		} else if !assertStringValue(&record.Network, info, networkIndex, "network", index, logger) {
			continue
		} else if !assertStringValue(&record.Deployment, info, deploymentIndex, "deployment", index, logger) {
			continue
		} else if !assertStringValue(&record.IP, info, ipIndex, "ip", index, logger) {
			continue
		} else if !assertStringArrayOfStringValue(&record.GroupIDs, info, groupIdsIndex, "group_ids", index, logger) {
			continue
		}

		assertStringValue(&record.AZID, info, azIDIndex, "az_id", index, logger)
		assertStringIntegerValue(&record.InstanceIndex, info, instanceIndexIndex, "instance_index", index, logger)

		s.Records = append(s.Records, &record)
	}
	s.loadUp()

	return s, nil
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

func assertStringValue(field *string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	if fieldIdx < 0 {
		return false
	}

	var ok bool
	*field, ok = info[fieldIdx].(string)

	if !ok {
		logger.Warn("RecordSet", "Value %d (%s) of record %d is not expected type of %s: %#+v", fieldIdx, fieldName, infoIdx, "string", info[fieldIdx])
	}

	return ok
}

func assertStringArrayOfStringValue(field *[]string, info []interface{}, fieldIdx int, fieldName string, infoIdx int, logger boshlog.Logger) bool {
	if fieldIdx < 0 {
		// *field = []string{}
		return true
	}

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

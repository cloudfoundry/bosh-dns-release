package records

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"strconv"

	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/criteria"
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/record"
	"bosh-dns/dns/server/tracker"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type recordGroup map[*record.Record]struct{}

type RecordSet struct {
	recordFileReader    FileReader
	recordsMutex        sync.RWMutex
	subscriberssMutex   sync.RWMutex
	subscribers         []chan bool
	logger              boshlog.Logger
	aliasList           aliases.Config
	healthWatcher       healthiness.HealthWatcher
	healthChan          chan record.Host
	trackerSubscription chan []record.Record

	domains           []string
	Records           []record.Record
	AgentAliasEnabled bool
}

func NewRecordSet(
	recordFileReader FileReader,
	aliasList aliases.Config,
	healthWatcher healthiness.HealthWatcher,
	maximumTrackedDomains uint,
	shutdownChan chan struct{},
	logger boshlog.Logger,
	AgentAliasEnabled bool,
) (*RecordSet, error) {
	r := &RecordSet{
		recordFileReader:    recordFileReader,
		logger:              logger,
		aliasList:           aliasList,
		healthWatcher:       healthWatcher,
		healthChan:          make(chan record.Host, 2),
		trackerSubscription: make(chan []record.Record),
		AgentAliasEnabled:   AgentAliasEnabled,
	}

	trackedDomains := tracker.NewPriorityLimitedTranscript(maximumTrackedDomains)
	tracker.Start(shutdownChan, r.trackerSubscription, r.healthChan, trackedDomains, healthWatcher, &QueryFilter{})

	r.update()

	go func() {
		subscriptionChan := recordFileReader.Subscribe()

		defer func() {
			r.subscriberssMutex.RLock()
			for _, subscriber := range r.subscribers {
				close(subscriber)
			}
			r.subscriberssMutex.RUnlock()
		}()

		for {
			select {
			case <-shutdownChan:
				return
			case ok := <-subscriptionChan:
				if !ok {
					return
				}

				r.update()

				r.subscriberssMutex.RLock()
				for _, subscriber := range r.subscribers {
					subscriber <- true
				}
				r.subscriberssMutex.RUnlock()
			}
		}
	}()

	return r, nil
}

func (r *RecordSet) Subscribe() <-chan bool {
	r.subscriberssMutex.Lock()
	defer r.subscriberssMutex.Unlock()
	c := make(chan bool)
	r.subscribers = append(r.subscribers, c)
	return c
}

func (r *RecordSet) Resolve(fqdn string) ([]string, error) {
	aliasExpansions := r.ExpandAliases(fqdn)
	finalRecords, err := r.Filter(aliasExpansions, true)
	if err != nil {
		return []string{}, err
	}

	finalIPs := make([]string, len(finalRecords))
	for i, rec := range finalRecords {
		finalIPs[i] = rec.IP
	}

	for _, expansion := range aliasExpansions {
		if net.ParseIP(expansion) != nil {
			finalIPs = append(finalIPs, expansion)
		}
	}
	return finalIPs, nil
}

func (r *RecordSet) ExpandAliases(fqdn string) []string {
	resolutions := r.aliasList.Resolutions(fqdn)
	if len(resolutions) == 0 {
		resolutions = []string{fqdn}
	}
	return resolutions
}

func (r *RecordSet) AllRecords() *[]record.Record {
	return &r.Records
}

func (r *RecordSet) HasIP(ip string) bool {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	for _, r := range r.Records {
		if r.IP == ip {
			return true
		}
	}
	return false
}

func (r *RecordSet) Filter(resolutions []string, shouldTrack bool) ([]record.Record, error) {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	var (
		finalRecords []record.Record
		errs         []error
	)

	qf := NewHealthFilter(&QueryFilter{}, r.healthChan, r.healthWatcher, shouldTrack)
	for _, resolution := range resolutions {
		crit, err := criteria.NewCriteria(resolution, r.domains)
		if err != nil {
			errs = append(errs, err)
		} else {
			results := qf.Filter(crit, r.Records)

			finalRecords = append(finalRecords, results...)
		}
	}

	if len(finalRecords) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("failures occurred when resolving alias domains: %s", errs)
	}

	return finalRecords, nil
}

func (r *RecordSet) Domains() []string {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	return append(r.domains, r.aliasList.AliasHosts()...)
}

func (r *RecordSet) update() {
	contents, err := r.recordFileReader.Get()
	if err != nil {
		return
	}
	records, err := createFromJSON(contents, r.logger)

	if err != nil {
		return
	}

	r.recordsMutex.Lock()
	defer r.recordsMutex.Unlock()

	r.Records = records

	if r.AgentAliasEnabled {
		for _, re := range r.Records {
			r.aliasList.SetAlias(fmt.Sprintf("%s.", re.AgentID), []string{
				fmt.Sprintf("%s.%s.%s.%s.%s", re.ID, re.Group, re.Network, re.Deployment, re.Domain),
			})
		}
	}

	r.trackerSubscription <- records

	domains := make(map[string]struct{})
	for _, record := range r.Records {
		domains[record.Domain] = struct{}{}
	}
	for domain := range domains {
		r.domains = append(r.domains, domain)
	}
}

func createFromJSON(j []byte, logger boshlog.Logger) ([]record.Record, error) {
	swap := struct {
		Keys  []string        `json:"record_keys"`
		Infos [][]interface{} `json:"record_infos"`
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		return nil, err
	}

	records := make([]record.Record, 0, len(swap.Infos))

	idIndex := -1
	numIDIndex := -1
	groupIndex := -1
	networkIndex := -1
	networkIDIndex := -1
	deploymentIndex := -1
	ipIndex := -1
	domainIndex := -1
	azIndex := -1
	azIDIndex := -1
	instanceIndexIndex := -1
	groupIdsIndex := -1
	agentIdIndex := -1

	for i, k := range swap.Keys {
		switch k {
		case "id":
			idIndex = i
		case "num_id":
			numIDIndex = i
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
		case "az":
			azIndex = i
		case "az_id":
			azIDIndex = i
		case "instance_index":
			instanceIndexIndex = i
		case "agent_id":
			agentIdIndex = i
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

		record := record.Record{Domain: domain}

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
		} else if !optionalStringValue(&record.AZ, info, azIndex, "az", index, logger) {
			continue
		} else if !optionalStringValue(&record.AZID, info, azIDIndex, "az_id", index, logger) {
			continue
		} else if !optionalStringValue(&record.NetworkID, info, networkIDIndex, "network_id", index, logger) {
			continue
		} else if !optionalStringValue(&record.NumID, info, numIDIndex, "num_id", index, logger) {
			continue
		} else if !optionalStringValue(&record.AgentID, info, agentIdIndex, "agent_id", index, logger) {
			continue
		} else if groupIdsIndex >= 0 && !assertStringArrayOfStringValue(&record.GroupIDs, info, groupIdsIndex, "group_ids", index, logger) {
			continue
		}

		assertStringIntegerValue(&record.InstanceIndex, info, instanceIndexIndex, "instance_index", index, logger)

		records = append(records, record)
	}

	return records, nil
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

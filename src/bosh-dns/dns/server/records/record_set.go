package records

import (
	"encoding/json"
	"errors"
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

type AliasDefinition struct {
	GroupID            string `json:"group_id"`
	RootDomain         string `json:"root_domain"`
	PlaceholderType    string `json:"placeholder_type"`
	HealthFilter       string `json:"health_filter"`
	InitialHealthCheck string `json:"initial_health_check"`
}

type recordGroup map[*record.Record]struct{}

var (
	CriteriaError = errors.New("error parsing query criteria")
	DomainError   = errors.New("no records match requested domain")
)

type RecordSet struct {
	recordFileReader    FileReader
	recordsMutex        sync.RWMutex
	subscriberssMutex   sync.RWMutex
	subscribers         []chan bool
	logger              boshlog.Logger
	aliasList           aliases.Config
	mergedAliasList     aliases.Config
	healthWatcher       healthiness.HealthWatcher
	healthChan          chan record.Host
	trackerSubscription chan []record.Record
	filtererFactory     FiltererFactory
	aliasQueryEncoder   AliasQueryEncoder

	domains []string
	records []record.Record
	hosts   []record.Host
	version uint64
}

func NewRecordSet(
	recordFileReader FileReader,
	aliasList aliases.Config,
	healthWatcher healthiness.HealthWatcher,
	maximumTrackedDomains uint,
	shutdownChan chan struct{},
	logger boshlog.Logger,
	filtererFactory FiltererFactory,
	AliasQueryEncoder AliasQueryEncoder,
) (*RecordSet, error) {
	r := &RecordSet{
		recordFileReader:    recordFileReader,
		logger:              logger,
		aliasList:           aliasList,
		aliasQueryEncoder:   AliasQueryEncoder,
		mergedAliasList:     aliases.NewConfig().Merge(aliasList),
		healthWatcher:       healthWatcher,
		healthChan:          make(chan record.Host, 2),
		trackerSubscription: make(chan []record.Record),
		filtererFactory:     filtererFactory,
	}

	trackedDomains := tracker.NewPriorityLimitedTranscript(maximumTrackedDomains)
	tracker.Start(shutdownChan, r.trackerSubscription, r.healthChan, trackedDomains, healthWatcher, filtererFactory.NewQueryFilterer(), logger)

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
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	aliasExpansions := r.unsafeExpandAliases(fqdn)
	r.logger.Debug("RecordSet", "Expand %s to %v", fqdn, aliasExpansions)

	aliasIPs := []string{}
	for _, expansion := range aliasExpansions {
		if net.ParseIP(expansion) != nil {
			aliasIPs = append(aliasIPs, expansion)
		}
	}

	finalRecords, err := r.unsafeResolveRecords(aliasExpansions, true)
	if err != nil {
		if !errors.Is(err, DomainError) || len(aliasIPs) == 0 {
			return nil, err
		}
	}

	finalIPs := make([]string, len(finalRecords))
	for i, rec := range finalRecords {
		finalIPs[i] = rec.IP
	}
	finalIPs = append(finalIPs, aliasIPs...)

	return finalIPs, nil
}

func (r *RecordSet) ResolveRecords(domains []string, shouldTrack bool) ([]record.Record, error) {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	return r.unsafeResolveRecords(domains, shouldTrack)
}

func (r *RecordSet) unsafeResolveRecords(domains []string, shouldTrack bool) ([]record.Record, error) {
	domainFilter := r.filtererFactory.NewQueryFilterer()
	healthFilter := r.filtererFactory.NewHealthFilterer(r.healthChan, shouldTrack)

	allCriteria, err := r.parseCriteria(domains)
	if err != nil {
		r.logger.Debug("RecordSet", "Error parsing domains %v: %v", domains, err)
		return nil, CriteriaError
	}
	domainRecords := r.filterRecords(domainFilter, allCriteria, r.records)
	if len(domainRecords) == 0 {
		r.logger.Debug("RecordSet", "No records match domains %v", domains)
		return nil, DomainError
	}

	finalRecords := r.filterRecords(healthFilter, allCriteria, domainRecords)
	if len(finalRecords) == 0 {
		r.logger.Debug("RecordSet", "No records match filter for domains %v", domains)
	}

	return finalRecords, nil
}

func (r *RecordSet) ExpandAliases(fqdn string) []string {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	return r.unsafeExpandAliases(fqdn)
}

func (r *RecordSet) unsafeExpandAliases(fqdn string) []string {
	resolutions := r.mergedAliasList.Resolutions(fqdn)
	if len(resolutions) == 0 {
		resolutions = []string{fqdn}
	}
	return resolutions
}

func (r *RecordSet) parseCriteria(resolutions []string) ([]criteria.Criteria, error) {
	crits := []criteria.Criteria{}

	for _, resolution := range resolutions {
		crit, err := criteria.NewCriteria(resolution, r.domains)
		if err != nil {
			return nil, err
		} else {
			crits = append(crits, crit)
		}
	}
	return crits, nil
}

func (r *RecordSet) filterRecords(filterer Filterer, filterCriteria []criteria.Criteria, records []record.Record) []record.Record {
	finalRecords := []record.Record{}

	for _, crit := range filterCriteria {
		results := filterer.Filter(crit, records)
		finalRecords = append(finalRecords, results...)
	}

	return finalRecords
}

func (r *RecordSet) AllRecords() []record.Record {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()
	return r.records
}

func (r *RecordSet) HasIP(ip string) bool {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	for _, r := range r.records {
		if r.IP == ip {
			return true
		}
	}
	return false
}

func (r *RecordSet) GetFQDNs(ip string) []string {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	uniqueFqnds := make(map[string]bool)

	for _, host := range r.hosts {
		if host.IP == ip {
			domain := dns.Fqdn(host.FQDN)
			uniqueFqnds[domain] = true
			for _, alias := range r.mergedAliasList.DomainResolutions(domain) {
				uniqueFqnds[alias] = true
			}

		}
	}
	fqdns := []string{}
	for domain, _ := range uniqueFqnds {
		fqdns = append(fqdns, domain)
	}
	r.logger.Debug("RecordSet", "Domains for %s: %v", ip, fqdns)
	return fqdns
}

func (r *RecordSet) Domains() []string {
	r.recordsMutex.RLock()
	defer r.recordsMutex.RUnlock()

	return append(r.domains, r.mergedAliasList.AliasHosts()...)
}

func (r *RecordSet) update() {
	contents, err := r.recordFileReader.Get()
	if err != nil {
		return
	}
	records, updatedAliases, hosts, version, err := createFromJSON(contents, r.logger, r.aliasQueryEncoder)
	if err != nil {
		return
	}

	r.recordsMutex.Lock()
	defer r.recordsMutex.Unlock()

	if r.version != version {
		r.logger.Info("RecordSet", "DNS blob updated from %d to %d", r.version, version)
	}

	r.version = version
	r.records = records
	r.hosts = hosts

	r.mergedAliasList = aliases.NewConfig().Merge(r.aliasList).Merge(updatedAliases)

	r.trackerSubscription <- records

	domains := make(map[string]struct{})
	for _, record := range r.records {
		domains[record.Domain] = struct{}{}
	}
	r.domains = make([]string, len(domains))
	i := 0
	for domain := range domains {
		r.domains[i] = domain
		i++
	}
}

//go:generate counterfeiter . AliasQueryEncoder
type AliasQueryEncoder interface {
	EncodeAliasesIntoQueries([]record.Record, map[string][]AliasDefinition) map[string][]string
}

func createFromJSON(j []byte, logger boshlog.Logger, aliasEncoder AliasQueryEncoder) ([]record.Record, aliases.Config, []record.Host, uint64, error) {
	swap := struct {
		Keys    []string                     `json:"record_keys"`
		Infos   [][]interface{}              `json:"record_infos"`
		Aliases map[string][]AliasDefinition `json:"aliases"`
		Version uint64                       `json:"Version"`
		Records [][2]string                  `json:"records"` // ip -> domain
	}{}

	err := json.Unmarshal(j, &swap)
	if err != nil {
		logger.Warn("RecordSet", "Unable to parse records file. Error: %v", err)
		return nil, aliases.NewConfig(), nil, 0, err
	}
	logger.Debug("RecordSet", "Read DNS blob version %d", swap.Version)

	records := make([]record.Record, 0, len(swap.Infos))
	hosts := make([]record.Host, 0, len(swap.Records))

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

	var updatedAliases aliases.Config
	aliasesToConfigure := aliasEncoder.EncodeAliasesIntoQueries(records, swap.Aliases)
	if updatedAliases, err = aliases.NewConfigFromMap(aliasesToConfigure); err != nil {
		logger.Warn("RecordSet", "Unable to configure aliases from records. Error: %v", err)
		// TODO: return records?
		return nil, aliases.NewConfig(), nil, 0, err
	}

	for _, hostArr := range swap.Records {
		hosts = append(hosts, record.Host{
			IP:   hostArr[0],
			FQDN: hostArr[1],
		})
	}

	return records, updatedAliases, hosts, swap.Version, nil
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

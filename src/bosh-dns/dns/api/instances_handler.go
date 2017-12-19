package api

import (
	"bosh-dns/dns/server/records"
	"encoding/json"
	"net/http"
)

//go:generate counterfeiter . HealthStateGetter

type HealthStateGetter interface {
	HealthState(ip string) string
}

//go:generate counterfeiter . RecordManager

type RecordManager interface {
	Filter(aliasExpansions []string, shouldTrack bool) ([]records.Record, error)
	AllRecords() *[]records.Record
	ExpandAliases(fqdn string) []string
}

type InstancesHandler struct {
	recordManager     RecordManager
	healthStateGetter HealthStateGetter
}

func NewInstancesHandler(recordManager RecordManager, healthStateGetter HealthStateGetter) *InstancesHandler {
	return &InstancesHandler{
		recordManager:     recordManager,
		healthStateGetter: healthStateGetter,
	}
}

func (h *InstancesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	var rs []records.Record
	if address == "" {
		rs = *h.recordManager.AllRecords()
	} else {
		var err error
		expandedAliases := h.recordManager.ExpandAliases(address)
		// h.recordSet.Records
		rs, err = h.recordManager.Filter(expandedAliases, false)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(err.Error()))
			return
		}

	}
	encoder := json.NewEncoder(w)
	for _, record := range rs {
		encoder.Encode(Record{
			ID:          record.ID,
			Group:       record.Group,
			Network:     record.Network,
			Deployment:  record.Deployment,
			IP:          record.IP,
			Domain:      record.Domain,
			AZ:          record.AZ,
			Index:       record.InstanceIndex,
			HealthState: h.healthStateGetter.HealthState(record.IP),
		})
	}
}

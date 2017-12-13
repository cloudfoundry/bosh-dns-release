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

type InstancesHandler struct {
	recordSet         *records.RecordSet
	healthStateGetter HealthStateGetter
}

func NewInstancesHandler(recordSet *records.RecordSet, healthStateGetter HealthStateGetter) *InstancesHandler {
	return &InstancesHandler{
		recordSet:         recordSet,
		healthStateGetter: healthStateGetter,
	}
}
func (h *InstancesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)

	for _, record := range h.recordSet.Records {
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

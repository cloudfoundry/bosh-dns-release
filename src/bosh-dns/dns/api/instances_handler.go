package api

import (
	"bosh-dns/dns/server/records"
	"encoding/json"
	"net/http"
)

type InstancesHandler struct {
	recordSet *records.RecordSet
}

func NewInstancesHandler(recordSet *records.RecordSet) *InstancesHandler {
	return &InstancesHandler{
		recordSet: recordSet,
	}
}
func (h *InstancesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)

	for _, record := range h.recordSet.Records {
		encoder.Encode(Record{
			ID:         record.ID,
			Group:      record.Group,
			Network:    record.Network,
			Deployment: record.Deployment,
			IP:         record.IP,
			Domain:     record.Domain,
			AZ:         record.AZ,
			Index:      record.InstanceIndex,
			Healthy:    true,
		})
	}
}

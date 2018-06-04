package api

import (
	"bosh-dns/dns/server/record"
	"encoding/json"
	"net/http"

	"github.com/miekg/dns"
)

//go:generate counterfeiter -o ./fakes/health_state_getter.go --fake-name HealthStateGetter . healthStateGetter
type healthStateGetter interface {
	HealthStateString(ip string) string
}

//go:generate counterfeiter -o ./fakes/record_manager.go --fake-name RecordManager . recordManager
type recordManager interface {
	Filter(aliasExpansions []string, shouldTrack bool) ([]record.Record, error)
	AllRecords() *[]record.Record
	ExpandAliases(fqdn string) []string
}

type InstancesHandler struct {
	recordManager     recordManager
	healthStateGetter healthStateGetter
}

func NewInstancesHandler(recordManager recordManager, healthStateGetter healthStateGetter) *InstancesHandler {
	return &InstancesHandler{
		recordManager:     recordManager,
		healthStateGetter: healthStateGetter,
	}
}

func (h *InstancesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	var rs []record.Record
	if address == "" {
		rs = *h.recordManager.AllRecords()
	} else {
		var err error
		address = dns.Fqdn(address)
		expandedAliases := h.recordManager.ExpandAliases(address)
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
			HealthState: h.healthStateGetter.HealthStateString(record.IP),
		})
	}
}

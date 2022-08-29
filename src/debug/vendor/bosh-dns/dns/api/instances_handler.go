package api

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"bosh-dns/dns/server/record"
	"encoding/json"
	"net/http"

	"github.com/miekg/dns"
)

//counterfeiter:generate -o ./fakes/health_state_getter.go . HealthStateGetter

type HealthStateGetter interface {
	HealthStateString(ip string) string
}

//counterfeiter:generate -o ./fakes/record_manager.go . RecordManager

type RecordManager interface {
	ResolveRecords(domains []string, shouldTrack bool) ([]record.Record, error)
	AllRecords() []record.Record
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
	var rs []record.Record
	if address == "" {
		rs = h.recordManager.AllRecords()
	} else {
		var err error
		address = dns.Fqdn(address)
		expandedAliases := h.recordManager.ExpandAliases(address)
		rs, err = h.recordManager.ResolveRecords(expandedAliases, false)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(err.Error())) //nolint:errcheck
			return
		}

	}
	encoder := json.NewEncoder(w)
	for _, record := range rs {
		encoder.Encode(InstanceRecord{ //nolint:errcheck
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

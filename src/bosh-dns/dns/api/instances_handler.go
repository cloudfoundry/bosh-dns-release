package api

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"net/http"

	"github.com/miekg/dns"

	"bosh-dns/dns/server/record"
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
	for _, rcd := range rs {
		encoder.Encode(InstanceRecord{ //nolint:errcheck
			ID:          rcd.ID,
			Group:       rcd.Group,
			Network:     rcd.Network,
			Deployment:  rcd.Deployment,
			IP:          rcd.IP,
			Domain:      rcd.Domain,
			AZ:          rcd.AZ,
			Index:       rcd.InstanceIndex,
			HealthState: h.healthStateGetter.HealthStateString(rcd.IP),
		})
	}
}

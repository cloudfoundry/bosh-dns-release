package api

import (
	"bosh-dns/healthcheck/api"
	"bosh-dns/healthconfig"
	"encoding/json"
	"net/http"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	GetStatus(ip string) api.HealthResult
}

type LocalGroupsHandler struct {
	jobs          []healthconfig.Job
	healthChecker HealthChecker
}

func NewLocalGroupsHandler(jobs []healthconfig.Job, hc HealthChecker) *LocalGroupsHandler {
	return &LocalGroupsHandler{
		jobs:          jobs,
		healthChecker: hc,
	}
}

func (h *LocalGroupsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	healthState := h.healthChecker.GetStatus("localhost")

	encoder.Encode(Group{ //nolint:errcheck
		HealthState: string(healthState.State),
	})

	for _, job := range h.jobs {
		for _, group := range job.Groups {
			encoder.Encode(Group{ //nolint:errcheck
				JobName:     group.JobName,
				LinkName:    group.Name,
				LinkType:    group.Type,
				GroupID:     group.Group,
				HealthState: string(healthState.GroupState[group.Group]),
			})
		}
	}
}

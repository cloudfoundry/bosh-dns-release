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

type GroupsHandler struct {
	jobs          []healthconfig.Job
	healthChecker HealthChecker
}

func NewGroupsHandler(jobs []healthconfig.Job, hc HealthChecker) *GroupsHandler {
	return &GroupsHandler{
		jobs:          jobs,
		healthChecker: hc,
	}
}

func (h *GroupsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	healthState := h.healthChecker.GetStatus("localhost")

	encoder.Encode(Group{
		HealthState: string(healthState.State),
	})

	for _, job := range h.jobs {
		for _, group := range job.Groups {
			encoder.Encode(Group{
				JobName:     group.JobName,
				LinkName:    group.Name,
				LinkType:    group.Type,
				GroupID:     group.Group,
				HealthState: string(healthState.GroupState[group.Group]),
			})
		}
	}
}

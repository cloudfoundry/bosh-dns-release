package api

type HealthStatus string

const (
	StatusRunning HealthStatus = "running"
	StatusFailing HealthStatus = "failing"
)

type HealthResult struct {
	State      HealthStatus            `json:"state"`
	GroupState map[string]HealthStatus `json:"group_state,omitempty"`
}

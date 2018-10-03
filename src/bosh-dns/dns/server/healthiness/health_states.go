package healthiness

import "bosh-dns/healthcheck/api"

type HealthState string

const (
	StateUnknown   api.HealthStatus = "unknown"
	StateUnchecked api.HealthStatus = "unchecked"
)

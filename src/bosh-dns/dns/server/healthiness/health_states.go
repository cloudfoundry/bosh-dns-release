package healthiness

type HealthState string

const (
	StateUnknown   HealthState = "unknown"
	StateHealthy   HealthState = "healthy"
	StateUnhealthy HealthState = "unhealthy"
	StateUnchecked HealthState = "unchecked"
)

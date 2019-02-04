package healthiness

import "bosh-dns/healthcheck/api"

type DisabledHealthChecker struct{}

func NewDisabledHealthChecker() *DisabledHealthChecker {
	return &DisabledHealthChecker{}
}

func (*DisabledHealthChecker) GetStatus(_ string) api.HealthResult {
	return api.HealthResult{}
}

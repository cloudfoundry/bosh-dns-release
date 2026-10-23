package healthiness

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-dns/healthcheck/api"
)

//counterfeiter:generate . HTTPClientGetter

type HTTPClientGetter interface {
	Get(endpoint string) (*http.Response, error)
}

// Health responses contain only status and group metadata.
const maxHealthResponseBytes = 1 << 20

type healthChecker struct {
	client HTTPClientGetter
	port   int
	logger boshlog.Logger
	logTag string
}

func NewHealthChecker(client HTTPClientGetter, port int, logger boshlog.Logger) HealthChecker {
	return &healthChecker{
		client: client,
		port:   port,
		logTag: "HealthChecker",
		logger: logger,
	}
}

type healthStatus struct { //nolint:unused
	State api.HealthStatus
}

func (hc *healthChecker) GetStatus(ip string) api.HealthResult {
	endpoint := fmt.Sprintf("https://%s/health", net.JoinHostPort(ip, fmt.Sprintf("%d", hc.port)))

	response, err := hc.client.Get(endpoint)
	if err != nil {
		hc.logger.Warn(hc.logTag, "network error connecting to %s: %v", ip, err)
		return api.HealthResult{State: StateUnknown}
	}
	defer response.Body.Close() //nolint:errcheck

	if response.StatusCode != http.StatusOK {
		hc.logger.Warn(hc.logTag, "http error connecting to %s: %v", ip, response.StatusCode)
		return api.HealthResult{State: StateUnknown}
	}

	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, maxHealthResponseBytes+1))
	if err != nil {
		hc.logger.Warn(hc.logTag, "error reading response body from %s: %v", ip, err)
		return api.HealthResult{State: StateUnknown} // untested
	}

	if len(responseBytes) > maxHealthResponseBytes {
		hc.logger.Warn(hc.logTag, "health response from %s exceeds size limit", ip)
		return api.HealthResult{State: StateUnknown}
	}

	var parsedResponse api.HealthResult
	err = json.Unmarshal(responseBytes, &parsedResponse)
	if err != nil {
		hc.logger.Warn(hc.logTag, "error parsing response body from %s: %v", ip, err)
		return api.HealthResult{State: StateUnknown}
	}

	hc.logger.Debug(hc.logTag, "health response from %s: %+v", ip, parsedResponse)

	return parsedResponse
}

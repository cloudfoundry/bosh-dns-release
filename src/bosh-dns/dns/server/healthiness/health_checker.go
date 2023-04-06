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

type healthStatus struct { //nolint:deadcode,unused
	State api.HealthStatus
}

func (hc *healthChecker) GetStatus(ip string) api.HealthResult {
	endpoint := fmt.Sprintf("https://%s/health", net.JoinHostPort(ip, fmt.Sprintf("%d", hc.port)))

	response, err := hc.client.Get(endpoint)
	if err != nil {
		hc.logger.Warn(hc.logTag, "network error connecting to %s: %v", ip, err)
		return api.HealthResult{State: StateUnknown}
	} else if response.StatusCode != http.StatusOK {
		hc.logger.Warn(hc.logTag, "http error connecting to %s: %v", ip, response.StatusCode)
		return api.HealthResult{State: StateUnknown}
	}

	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		hc.logger.Warn(hc.logTag, "error reading response body from %s: %v", ip, err)
		return api.HealthResult{State: StateUnknown} // untested
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

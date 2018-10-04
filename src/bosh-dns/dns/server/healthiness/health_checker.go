package healthiness

import (
	"bosh-dns/healthcheck/api"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
)

//go:generate counterfeiter . HTTPClientGetter

type HTTPClientGetter interface {
	Get(endpoint string) (*http.Response, error)
}

type healthChecker struct {
	client HTTPClientGetter
	port   int
}

func NewHealthChecker(client HTTPClientGetter, port int) HealthChecker {
	return &healthChecker{
		client: client,
		port:   port,
	}
}

type healthStatus struct {
	State api.HealthStatus
}

func (hc *healthChecker) GetStatus(ip string) api.HealthResult {
	endpoint := fmt.Sprintf("https://%s/health", net.JoinHostPort(ip, fmt.Sprintf("%d", hc.port)))

	response, err := hc.client.Get(endpoint)
	if err != nil {
		return api.HealthResult{State: StateUnknown}
	} else if response.StatusCode != http.StatusOK {
		return api.HealthResult{State: StateUnknown}
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return api.HealthResult{State: StateUnknown} // untested
	}

	var parsedResponse api.HealthResult
	err = json.Unmarshal(responseBytes, &parsedResponse)
	if err != nil {
		return api.HealthResult{State: StateUnknown}
	}

	return parsedResponse
}

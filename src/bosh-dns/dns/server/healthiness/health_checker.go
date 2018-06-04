package healthiness

import (
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
	State string
}

func (hc *healthChecker) GetStatus(ip string) HealthState {
	endpoint := fmt.Sprintf("https://%s/health", net.JoinHostPort(ip, fmt.Sprintf("%d", hc.port)))

	response, err := hc.client.Get(endpoint)
	if err != nil {
		return StateUnknown
	} else if response.StatusCode != http.StatusOK {
		return StateUnknown
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return StateUnknown // untested
	}

	var parsedResponse healthStatus
	err = json.Unmarshal(responseBytes, &parsedResponse)
	if err != nil {
		return StateUnknown
	}

	if parsedResponse.State == "running" {
		return StateHealthy
	}

	return StateUnhealthy
}

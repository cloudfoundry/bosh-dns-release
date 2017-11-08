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

func (hc *healthChecker) GetStatus(ip string) bool {
	endpoint := fmt.Sprintf("https://%s/health", net.JoinHostPort(ip, fmt.Sprintf("%d", hc.port)))

	response, err := hc.client.Get(endpoint)
	if err != nil {
		return false
	} else if response.StatusCode != 200 {
		return false
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false // untested
	}

	var parsedResponse healthStatus
	_ = json.Unmarshal(responseBytes, &parsedResponse)

	return parsedResponse.State == "running"
}

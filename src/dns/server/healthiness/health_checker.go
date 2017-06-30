package healthiness

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/cloudfoundry/bosh-utils/httpclient"
)

type healthChecker struct {
	client httpclient.HTTPClient
	port   int
}

func NewHealthChecker(client httpclient.HTTPClient, port int) HealthChecker {
	return &healthChecker{
		client: client,
		port:   port,
	}
}

type healthStatus struct {
	State string
}

func (hc *healthChecker) GetStatus(ip string) bool {
	endpoint := fmt.Sprintf("https://%s:%d/health", ip, hc.port)

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

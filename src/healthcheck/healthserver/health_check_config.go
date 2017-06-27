package healthserver

type HealthCheckConfig struct {
	Port           int    `json:"port"`
	CertFile       string `json:"certFile"`
	KeyFile        string `json:"keyFile"`
	CaFile         string `json:"caFile"`
	HealthFileName string `json:"healthFileName"`
}

const CN = "health.bosh-dns"

package healthserver

import "bosh-dns/dns/config"

type HealthCheckConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`

	CAFile          string `json:"ca_file"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`

	HealthExecutableInterval config.DurationJSON `json:"health_executable_interval"`
	HealthExecutablesGlob    string              `json:"health_executables_glob"`
	HealthFileName           string              `json:"health_file_name"`
}

const CN = "health.bosh-dns"

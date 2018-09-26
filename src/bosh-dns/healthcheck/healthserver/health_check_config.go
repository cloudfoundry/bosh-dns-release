package healthserver

import "bosh-dns/dns/config"

type HealthCheckConfig struct {
	Address                  string              `json:"address"`
	Port                     int                 `json:"port"`
	CertificateFile          string              `json:"certificate_file"`
	PrivateKeyFile           string              `json:"private_key_file"`
	CAFile                   string              `json:"ca_file"`
	HealthFileName           string              `json:"health_file_name"`
	RecordsFileName          string              `json:"records_file_name"`
	HealthExecutablesGlob    string              `json:"health_executables_glob"`
	HealthExecutableInterval config.DurationJSON `json:"health_executable_interval"`
}

const CN = "health.bosh-dns"

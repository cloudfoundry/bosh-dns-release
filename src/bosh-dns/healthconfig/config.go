package healthconfig

import "bosh-dns/dns/config"

type HealthCheckConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`

	CAFile          string `json:"ca_file"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`

	HealthExecutableInterval config.DurationJSON `json:"health_executable_interval"`
	HealthExecutablePath     string              `json:"health_executable_path"`
	HealthFileName           string              `json:"health_file_name"`

	JobsDir string `json:"jobs_dir"`

	RequestTimeout config.DurationJSON `json:"request_timeout"`

	LogLevel string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

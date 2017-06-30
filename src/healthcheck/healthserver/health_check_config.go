package healthserver

type HealthCheckConfig struct {
	Port            int    `json:"port"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	CAFile          string `json:"ca_file"`
	HealthFileName  string `json:"health_file_name"`
}

const CN = "health.bosh-dns"

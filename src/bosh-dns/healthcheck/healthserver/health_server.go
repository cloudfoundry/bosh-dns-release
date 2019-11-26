package healthserver

import (
	"bosh-dns/healthcheck/api"
	"bosh-dns/healthconfig"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"code.cloudfoundry.org/tlsconfig"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const CN = "health.bosh-dns"

type HealthServer interface {
	Serve(config *healthconfig.HealthCheckConfig)
}

type HealthExecutable interface {
	Status() api.HealthResult
}

type concreteHealthServer struct {
	logger           boshlog.Logger
	healthExecutable HealthExecutable
}

const logTag = "healthServer"

func NewHealthServer(logger boshlog.Logger, healthFileName string, healthExecutable HealthExecutable) HealthServer {
	return &concreteHealthServer{
		logger:           logger,
		healthExecutable: healthExecutable,
	}
}

func (c *concreteHealthServer) Serve(config *healthconfig.HealthCheckConfig) {
	http.HandleFunc("/health", c.healthEntryPoint)

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithIdentityFromFile(config.CertificateFile, config.PrivateKeyFile),
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(config.CAFile),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	tlsConfig.BuildNameToCertificate()

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", config.Address, config.Port),
		TLSConfig: tlsConfig,
	}
	server.SetKeepAlivesEnabled(false)

	serveErr := server.ListenAndServeTLS("", "")
	c.logger.Error(logTag, "http healthcheck ending with %s", serveErr)
}

func (c *concreteHealthServer) healthEntryPoint(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Should not be possible to get here without having a peer certificate
	cn := r.TLS.PeerCertificates[0].Subject.CommonName
	if cn != CN {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Add("Content-Type", "text/plain")
		_, err := w.Write([]byte("TLS certificate common name does not match"))
		if err != nil {
			c.logger.Error(logTag, "failed to write healthcheck status data: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	w.Header().Add("Content-Type", "application/json")

	status := c.healthExecutable.Status()
	statusData, err := json.Marshal(status)
	if err != nil {
		c.logger.Error(logTag, "failed to marshal healthcheck data: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(statusData)
	if err != nil {
		c.logger.Error(logTag, "failed to write healthcheck status data: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

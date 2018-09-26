package healthserver

import (
	"bosh-dns/healthcheck/healthexecutable"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
)

type HealthServer interface {
	Serve(config *HealthCheckConfig)
}

type HealthExecutable interface {
	Status() healthexecutable.Status
}

type concreteHealthServer struct {
	logger             boshlog.Logger
	fs                 system.FileSystem
	healthJsonFileName string
	healthExecutable   HealthExecutable
}

type healthResponse struct {
	State      string            `json:"state"`
	GroupState map[string]string `json:"group_state"`
}

const logTag = "healthServer"

func NewHealthServer(logger boshlog.Logger, fs system.FileSystem, healthFileName string, healthExecutable HealthExecutable) HealthServer {
	return &concreteHealthServer{
		logger:             logger,
		fs:                 fs,
		healthJsonFileName: healthFileName,
		healthExecutable:   healthExecutable,
	}
}

func (c *concreteHealthServer) Serve(config *HealthCheckConfig) {
	http.HandleFunc("/health", c.healthEntryPoint)

	caCert, err := ioutil.ReadFile(config.CAFile)
	if err != nil {
		log.Fatal(err)
		return
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(config.CertificateFile, config.PrivateKeyFile)
	if err != nil {
		log.Fatal(err)
		return
	}

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithIdentity(cert),
		tlsconfig.WithInternalServiceDefaults(),
	)

	serverConfig := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))
	serverConfig.BuildNameToCertificate()

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", config.Address, config.Port),
		TLSConfig: serverConfig,
	}
	server.SetKeepAlivesEnabled(false)

	serveErr := server.ListenAndServeTLS("", "")
	c.logger.Error(logTag, "http healthcheck ending with %s", serveErr)
}

func (c *concreteHealthServer) healthEntryPoint(w http.ResponseWriter, r *http.Request) {
	// Should not be possible to get here without having a peer certificate
	cn := r.TLS.PeerCertificates[0].Subject.CommonName
	if cn != CN {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("TLS certificate common name does not match"))
		return
	}

	w.Header().Add("Content-Type", "application/json")

	status := c.healthExecutable.Status()

	var statusBoolToString = map[bool]string{true: "running", false: "stopped"}

	healthResponse := healthResponse{
		State:      statusBoolToString[status.VmStatus],
		GroupState: map[string]string{},
	}

	for groupName, groupStatus := range status.GroupStatus {
		healthResponse.GroupState[groupName] = statusBoolToString[groupStatus]
	}

	responseBytes, err := json.Marshal(healthResponse)
	if err != nil {
		c.logger.Error(logTag, "Failed to marshal response data %s. error: %s", responseBytes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(responseBytes)
}

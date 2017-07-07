package healthserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
)

type HealthServer interface {
	Serve(config *HealthCheckConfig)
}

type concreteHealthServer struct {
	logger             boshlog.Logger
	fs                 system.FileSystem
	healthJsonFileName string
}

const logTag = "healthServer"

func NewHealthServer(logger boshlog.Logger, fs system.FileSystem, healthFileName string) HealthServer {
	return &concreteHealthServer{
		logger:             logger,
		fs:                 fs,
		healthJsonFileName: healthFileName,
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
		Addr:      fmt.Sprintf(":%d", config.Port),
		TLSConfig: serverConfig,
	}

	c.logger.Error(logTag, "http healthcheck ending with %s", server.ListenAndServeTLS("", ""))
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

	healthRaw, err := ioutil.ReadFile(c.healthJsonFileName)

	if err != nil {
		c.logger.Error(logTag, "Couldn't marshall healthcheck data %s. error: %s", string(healthRaw), err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(healthRaw)
}

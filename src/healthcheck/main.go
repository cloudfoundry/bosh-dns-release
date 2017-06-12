package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"crypto/tls"
	"crypto/x509"
	"errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	pivTls "github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
	"io/ioutil"
	"log"
)

type HealthCheckConfig struct {
	Port     int    `json:"port"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
	CaFile   string `json:"caFile"`
}

var (
	logger boshlog.Logger
	logTag string
)

const CN = "health.bosh-dns"

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, os.Stdout, os.Stderr)
	logTag = "main"
	defer logger.FlushTimeout(5 * time.Second)

	config, err := getConfig()
	if err != nil {
		logger.Error(logTag, fmt.Sprintf("%s", err))
		return 1
	}

	serve(config)

	return 0
}

func serve(config *HealthCheckConfig) {
	http.HandleFunc("/health", healthEntryPoint)

	caCert, err := ioutil.ReadFile(config.CaFile)
	if err != nil {
		log.Fatal(err)
		return
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		log.Fatal(err)
		return
	}

	pConfig := pivTls.Build(
		pivTls.WithIdentity(cert),
		pivTls.WithPivotalDefaults(),
	)

	tlsConfig := pConfig.Server(pivTls.WithClientAuthentication(caCertPool))
	tlsConfig.BuildNameToCertificate()

	fmt.Printf("serverside: %#v\n", tlsConfig.CipherSuites)

	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", config.Port),
		TLSConfig: tlsConfig,
	}

	logger.Error(logTag, "http healthcheck ending with %s", server.ListenAndServeTLS("", ""))
}

func getConfig() (*HealthCheckConfig, error) {
	var configFile string
	var config *HealthCheckConfig

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	} else {
		configFile = "health.config.json"
	}

	configRaw, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't open config file for health. error: %s", err))
	}

	config = &HealthCheckConfig{}
	err = json.Unmarshal(configRaw, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't decode config file for health. error: %s", err))
	}

	return config, nil
}

func healthEntryPoint(w http.ResponseWriter, r *http.Request) {
	// Should not be possible to get here without having a peer certificate
	cn := r.TLS.PeerCertificates[0].Subject.CommonName
	if cn != CN {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("TLS certificate common name does not match"))
		return
	}

	ret := map[string]string{
		"state": "running",
	}

	fmt.Printf("Certificate: %#v", r.TLS.PeerCertificates[0].Subject.CommonName)

	asJson, err := json.Marshal(ret)
	if err != nil {
		logger.Error(logTag, "Couldn't marshall healthcheck data %s. error: %s", ret, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(asJson)
}

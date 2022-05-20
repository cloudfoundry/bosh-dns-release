package tlsclient

import (
	"crypto/tls"
	"io/ioutil"
	"time"

	"net/http"

	"crypto/x509"

	"code.cloudfoundry.org/tlsconfig"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func NewFromFiles(dnsName string, caFile, clientCertFile, clientKeyFile string, timeout time.Duration, logger boshlog.Logger) (*httpclient.HTTPClient, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	return New(dnsName, caCert, cert, timeout, logger)
}

func New(dnsName string, caCert []byte, cert tls.Certificate, timeout time.Duration, logger boshlog.Logger) (*httpclient.HTTPClient, error) {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithIdentity(cert),
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthority(caCertPool),
		tlsconfig.WithServerName(dnsName),
	)
	if err != nil {
		return nil, err
	}
	tlsConfig.BuildNameToCertificate()
	tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(10000)

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: transport}
	client.Timeout = timeout

	return httpclient.NewHTTPClient(
		client,
		logger,
	), nil
}

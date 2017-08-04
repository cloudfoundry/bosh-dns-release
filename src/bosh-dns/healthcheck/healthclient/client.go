package healthclient

import (
	"crypto/tls"
	"io/ioutil"
	"time"

	"net/http"

	"crypto/x509"

	boshhttp "github.com/cloudfoundry/bosh-utils/http"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func NewHealthClientFromFiles(caFile, clientCertFile, clientKeyFile string, logger boshlog.Logger) (httpclient.HTTPClient, error) {
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

	return NewHealthClient(caCert, cert, logger), nil
}

func NewHealthClient(caCert []byte, cert tls.Certificate, logger boshlog.Logger) httpclient.HTTPClient {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := boshhttp.NewMutualTLSClient(cert, caCertPool, "health.bosh-dns")
	client.Timeout = 5 * time.Second

	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.TLSClientConfig.ClientSessionCache = tls.NewLRUClientSessionCache(0)
	}
	httpClient := boshhttp.NewNetworkSafeRetryClient(client, 4, 500*time.Millisecond, logger)
	return httpclient.NewHTTPClient(httpClient, logger)
}

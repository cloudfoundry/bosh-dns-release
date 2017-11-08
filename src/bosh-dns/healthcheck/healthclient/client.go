package healthclient

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"time"

	"net/http"

	"crypto/x509"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func NewHealthClientFromFiles(caFile, clientCertFile, clientKeyFile string, logger boshlog.Logger) (*httpclient.HTTPClient, error) {
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

func NewHealthClient(caCert []byte, cert tls.Certificate, logger boshlog.Logger) *httpclient.HTTPClient {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := httpclient.NewMutualTLSClient(cert, caCertPool, "")
	client.Timeout = 5 * time.Second

	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.TLSClientConfig.ClientSessionCache = tls.NewLRUClientSessionCache(10000)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.TLSClientConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			certs := make([]*x509.Certificate, len(rawCerts))
			for i, asn1Data := range rawCerts {
				cert, err := x509.ParseCertificate(asn1Data)
				if err != nil {
					return errors.New("tls: failed to parse certificate from server: " + err.Error())
				}
				certs[i] = cert
			}

			opts := x509.VerifyOptions{
				Roots:         tr.TLSClientConfig.RootCAs,
				CurrentTime:   time.Now(),
				DNSName:       "health.bosh-dns",
				Intermediates: x509.NewCertPool(),
			}

			for i, cert := range certs {
				if i == 0 {
					continue
				}
				opts.Intermediates.AddCert(cert)
			}
			_, err := certs[0].Verify(opts)
			return err
		}
	}

	return httpclient.NewHTTPClient(
		httpclient.NewNetworkSafeRetryClient(client, 4, 500*time.Millisecond, logger),
		logger,
	)
}

#!/usr/bin/env bash

set -e

# Generate a new CSR and then certificate for the test CA
openssl x509 -in bosh-dns-test-ca.crt -signkey bosh-dns-test-ca.key -x509toreq -out bosh-dns-test-ca.csr
openssl x509 -signkey bosh-dns-test-ca.key -in bosh-dns-test-ca.csr -req -days 3650 -out bosh-dns-test-ca.crt

# Generate a new CSR and then sign the test API certificate using the test CA
openssl x509 -in bosh-dns-test-api-certificate.crt -signkey bosh-dns-test-api-certificate.key -x509toreq -out bosh-dns-test-api-certificate.csr
openssl x509 -CAcreateserial -CA bosh-dns-test-ca.crt -CAkey bosh-dns-test-ca.key  -in bosh-dns-test-api-certificate.csr -req -days 3650 -out bosh-dns-test-api-certificate.crt
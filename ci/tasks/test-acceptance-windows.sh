#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/bbl-state

export PATH=$BBL_STATE_DIR/bin:$PATH
source $BBL_STATE_DIR/bosh.sh

bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance

pushd $ROOT_DIR/dns-release
   bosh create-release --force && bosh upload-release --rebase
popd

bosh -n deploy $ROOT_DIR/dns-release/src/bosh-dns/test_yml_assets/windows-acceptance-manifest.yml \
  --var-file health_ca="${ROOT_DIR}/dns-release/src/bosh-dns/healthcheck/assets/test_certs/test_ca.pem" \
  --var-file health_tls_cert="${ROOT_DIR}/dns-release/src/bosh-dns/healthcheck/assets/test_certs/test_server.pem" \
  --var-file health_tls_key="${ROOT_DIR}/dns-release/src/bosh-dns/healthcheck/assets/test_certs/test_server.key" \
  --var-file client_health_tls_cert="${ROOT_DIR}/dns-release/src/bosh-dns/healthcheck/assets/test_certs/test_client.pem" \
  --var-file client_health_tls_key="${ROOT_DIR}/dns-release/src/bosh-dns/healthcheck/assets/test_certs/test_client.key" \
  -v health_server_port=2345 \
  -o $ROOT_DIR/dns-release/src/bosh-dns/healthcheck/assets/enable-health-windows-manifest-ops.yml

bosh run-errand acceptance-tests-windows --keep-alive

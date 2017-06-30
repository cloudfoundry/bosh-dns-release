#!/usr/bin/env bash

set -exu

ROOT_DIR=$PWD

start-bosh -o /usr/local/bosh-deployment/local-dns.yml

source /tmp/local-bosh/director/env

bosh int /tmp/local-bosh/director/creds.yml --path /jumpbox_ssh/private_key > /tmp/jumpbox_ssh_key.pem
chmod 400 /tmp/jumpbox_ssh_key.pem

export BOSH_GW_PRIVATE_KEY="/tmp/jumpbox_ssh_key.pem"
export BOSH_GW_USER="jumpbox"
export BOSH_DIRECTOR_IP="10.245.0.3"
export BOSH_BINARY_PATH=$(which bosh)
export BOSH_DEPLOYMENT="bosh-dns"
export TEST_CLOUD_CONFIG_PATH="/tmp/cloud-config.yml"
export TEST_MANIFEST_NAME="manifest"
export NO_RECURSORS_OPS_FILE="no-recursors-configured"
export LOCAL_RECURSOR_OPS_FILE="add-test-dns-nameservers"
export TEST_TARGET_OS="linux"

bosh int /usr/local/bosh-deployment/docker/cloud-config.yml \
    -o $ROOT_DIR/dns-release/src/test_yml_assets/add-static-ips-to-cloud-config.yml > ${TEST_CLOUD_CONFIG_PATH}

bosh -n update-cloud-config ${TEST_CLOUD_CONFIG_PATH} -v network=director_network

bosh upload-stemcell bosh-candidate-stemcell/bosh-stemcell-*.tgz

pushd $ROOT_DIR/dns-release
   bosh create-release --force && bosh upload-release
popd

export GOPATH=$PWD/dns-release
export PATH="${GOPATH}/bin":$PATH

go install vendor/github.com/onsi/ginkgo/ginkgo

pushd $GOPATH/src/acceptance_tests
    ginkgo -keepGoing -randomizeAllSpecs -randomizeSuites -race .
popd

bosh -n deploy $ROOT_DIR/dns-release/src/test_yml_assets/manifest.yml \
   -v acceptance_release_path=$ROOT_DIR/dns-release/src/acceptance_tests/dns-acceptance-release \
   --var-file health_ca="${ROOT_DIR}/dns-release/src/healthcheck/assets/test_certs/test_ca.pem" \
   --var-file health_tls_cert="${ROOT_DIR}/dns-release/src/healthcheck/assets/test_certs/test_server.pem" \
   --var-file health_tls_key="${ROOT_DIR}/dns-release/src/healthcheck/assets/test_certs/test_server.key" \
   -v health_server_port=2345 \
   -o $ROOT_DIR/dns-release/src/test_yml_assets/use-dns-release-default-bind-and-alias-addresses.yml \
   -o $ROOT_DIR/dns-release/src/healthcheck/assets/enable-health-manifest-ops.yml

pushd $GOPATH/src/acceptance_tests/linux
   ginkgo -keepGoing -randomizeAllSpecs -randomizeSuites -race -r .
popd

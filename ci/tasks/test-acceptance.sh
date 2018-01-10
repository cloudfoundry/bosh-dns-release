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
export BASE_STEMCELL="ubuntu-trusty"

bosh int /usr/local/bosh-deployment/docker/cloud-config.yml \
    -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/add-static-ips-to-cloud-config.yml > ${TEST_CLOUD_CONFIG_PATH}

bosh -n update-cloud-config ${TEST_CLOUD_CONFIG_PATH} -v network=director_network

bosh upload-stemcell bosh-candidate-stemcell/bosh-stemcell-*.tgz

bosh upload-release $ROOT_DIR/candidate-release/*.tgz

export GOPATH=$PWD/bosh-dns-release
export PATH="${GOPATH}/bin":$PATH

go install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

pushd $GOPATH/src/bosh-dns/acceptance_tests
    ginkgo -keepGoing -randomizeAllSpecs -randomizeSuites -race .
popd

bosh -n deploy $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/manifests/dns-linux.yml \
   -v acceptance_release_path=$ROOT_DIR/bosh-dns-release/src/bosh-dns/acceptance_tests/dns-acceptance-release \
   -v health_server_port=2345 \
   -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/use-dns-release-default-bind-and-alias-addresses.yml \
   -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/enable-health-manifest-ops.yml \
   -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/enable-require-dns-in-pre-start-ops.yml \
   --vars-store dns-creds.yml

pushd $GOPATH/src/bosh-dns/acceptance_tests/linux
   ginkgo -keepGoing -randomizeAllSpecs -randomizeSuites -race -r .
popd

#!/usr/bin/env bash

set -exu

ROOT_DIR=$PWD

start-bosh \
    -o /usr/local/bosh-deployment/local-bosh-release.yml \
    -o /usr/local/bosh-deployment/local-dns.yml \
    -v local_bosh_release=$PWD/bosh-candidate-release/bosh-dev-release.tgz

source /tmp/local-bosh/director/env

bosh int /tmp/local-bosh/director/creds.yml --path /jumpbox_ssh/private_key > /tmp/jumpbox_ssh_key.pem
chmod 400 /tmp/jumpbox_ssh_key.pem

export BOSH_GW_PRIVATE_KEY="/tmp/jumpbox_ssh_key.pem"
export BOSH_GW_USER="jumpbox"
export BOSH_DIRECTOR_IP="10.245.0.3"
export BOSH_BINARY_PATH=$(which bosh)
export BOSH_DEPLOYMENT="bosh-dns"

bosh int /usr/local/bosh-deployment/docker/cloud-config.yml \
    -o $ROOT_DIR/dns-release/ci/assets/add-static-ips-to-cloud-config.yml > /tmp/cloud-config.yml

bosh -n update-cloud-config /tmp/cloud-config.yml -v network=director_network

bosh upload-stemcell bosh-candidate-stemcell/bosh-stemcell-*.tgz
bosh -n deploy -v dns_release_path=$ROOT_DIR/dns-release $ROOT_DIR/dns-release/ci/assets/manifest.yml

export GOPATH=${ROOT_DIR}/go
export PATH="${GOPATH}/bin":$PATH

mkdir -p go/src/github.com/cloudfoundry
mkdir -p go/src/github.com/onsi
ln -s $PWD/dns-release $PWD/go/src/github.com/cloudfoundry/dns-release
ln -s $PWD/dns-release/src/vendor/github.com/onsi/ginkgo $PWD/go/src/github.com/onsi/ginkgo

go install github.com/onsi/ginkgo/ginkgo

echo $ZONES_JSON_HASH > /tmp/zones.json

pushd $GOPATH/src/github.com/cloudfoundry/dns-release/src/performance_tests
    ginkgo -r -randomizeAllSpecs -randomizeSuites -race .
popd

#!/usr/bin/env bash

set -exu

# Target the director
start-bosh \
    -o /usr/local/bosh-deployment/local-bosh-release-tarball.yml \
    -o /usr/local/bosh-deployment/local-dns.yml \
    -v local_bosh_release="$(echo -n "$PWD"/bosh-candidate-release/*.tgz)"

# shellcheck disable=SC1091
source /tmp/local-bosh/director/env

bosh int /tmp/local-bosh/director/creds.yml --path /jumpbox_ssh/private_key > /tmp/jumpbox_ssh_key.pem
chmod 400 /tmp/jumpbox_ssh_key.pem

export BOSH_GW_PRIVATE_KEY="/tmp/jumpbox_ssh_key.pem"
export BOSH_GW_USER="jumpbox"
export BOSH_DIRECTOR_IP="10.245.0.3"
export TEST_CLOUD_CONFIG_PATH="/tmp/cloud-config.yml"

# Configure the director for this test suite
bosh int /usr/local/bosh-deployment/docker/cloud-config.yml \
    -o "$PWD/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/add-static-ips-to-cloud-config.yml" \
    > "${TEST_CLOUD_CONFIG_PATH}"
bosh -n update-cloud-config "${TEST_CLOUD_CONFIG_PATH}" -v network=director_network
bosh upload-stemcell bosh-stemcell/*.tgz

bosh upload-release "$PWD"/candidate-release/*.tgz

pushd "$PWD/bosh-dns-release"
  ./scripts/run-acceptance-tests
popd


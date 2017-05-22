#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/bbl-state

export PATH=$BBL_STATE_DIR/bin:$PATH
source $BBL_STATE_DIR/bosh.sh

bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz
bosh -n upload-stemcell $ROOT_DIR/gcp-linux-stemcell/*.tgz

bosh -n deploy $ROOT_DIR/dns-release/ci/assets/shared-acceptance-manifest.yml \
    -v dns_release_path=$ROOT_DIR/dns-release \
    --var-file bosh_ca_cert=<(echo "$BOSH_CA_CERT") \
    -v bosh_client_secret="$BOSH_CLIENT_SECRET" \
    -v bosh_client="$BOSH_CLIENT" \
    -v bosh_environment="$BOSH_ENVIRONMENT" \
    -v bosh_deployment=bosh-dns

bosh run-errand acceptance-tests

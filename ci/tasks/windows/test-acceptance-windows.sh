#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

source $BBL_STATE_DIR/.envrc

bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance

bosh upload-release $ROOT_DIR/candidate-release/*.tgz

bosh -n deploy $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/manifests/windows-acceptance-manifest.yml \
  -v health_server_port=2345 \
  -v windows_stemcell=$WINDOWS_OS_VERSION \
  -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/enable-health-manifest-ops.yml \
  --vars-store dns-creds.yml

bosh run-errand acceptance-tests-windows

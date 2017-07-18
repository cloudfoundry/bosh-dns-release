#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

source $BBL_STATE_DIR/bosh.sh

bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance

pushd $ROOT_DIR/dns-release
   bosh create-release --force && bosh upload-release --rebase
popd

bosh -n deploy $ROOT_DIR/dns-release/src/bosh-dns/test_yml_assets/windows-acceptance-manifest.yml \
  -v health_server_port=2345 \
  -o $ROOT_DIR/dns-release/src/bosh-dns/test_yml_assets/enable-health-manifest-ops.yml \
  --vars-store dns-creds.yml

bosh run-errand acceptance-tests-windows --keep-alive

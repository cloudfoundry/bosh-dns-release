#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

pushd "${BBL_STATE_DIR}"
  source .envrc
popd

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance

bosh -n deploy \
  $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/manifests/windows-acceptance-manifest.yml \
  -v deployment_name="$BOSH_DEPLOYMENT" \
  -v windows_stemcell=$WINDOWS_OS_VERSION \
  --vars-store dns-creds.yml \
  -o $ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/ops/enable-health-manifest-windows.yml \
  -v health_server_port=2345

bosh run-errand acceptance-tests-windows

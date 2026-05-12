#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

ROOT_DIR=$PWD

source "${BBL_STATE_DIR}/.envrc"

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance-nameserver-disabled

bosh -n deploy \
  "$ROOT_DIR/bosh-dns-release/src/bosh-dns/test_yml_assets/manifests/windows-acceptance-manifest.yml" \
  -v deployment_name="$BOSH_DEPLOYMENT" \
  -v windows_stemcell="$WINDOWS_OS_VERSION" \
  --vars-store dns-creds.yml \
  -o "$ROOT_DIR/bosh-dns-release/src/bosh-dns/acceptance_tests/windows/disable_nameserver_override/manifest-ops.yml"

bosh run-errand acceptance-tests-windows

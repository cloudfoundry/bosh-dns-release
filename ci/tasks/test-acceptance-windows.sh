#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/bbl-state

export PATH=$BBL_STATE_DIR/bin:$PATH
source $BBL_STATE_DIR/bosh.sh

bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz

export BOSH_DEPLOYMENT=bosh-dns-windows-acceptance

pushd $ROOT_DIR/dns-release
   bosh create-release --force && bosh upload-release
popd

bosh -n deploy $ROOT_DIR/dns-release/src/test_yml_assets/windows-acceptance-manifest.yml

bosh run-errand acceptance-tests-windows

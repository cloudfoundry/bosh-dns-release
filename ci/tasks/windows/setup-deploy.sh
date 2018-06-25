#!/bin/bash -lx

set -eu -o pipefail

main() {
  ROOT_DIR=$PWD
  BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

  pushd $BBL_STATE_DIR
    source .envrc
  popd

  env

  bosh upload-release $ROOT_DIR/candidate-release/*.tgz
  bosh -n upload-stemcell $ROOT_DIR/bosh-stemcell-windows/*.tgz
  bosh -n upload-stemcell $ROOT_DIR/gcp-linux-stemcell/*.tgz
}

main

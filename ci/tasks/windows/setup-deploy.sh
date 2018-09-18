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

  # Need to delete the bosh-dns runtime config because bbl uses a hard-coded
  # bosh-deployment which specifies a bosh-dns version that may conflict with the
  # one we are trying to test.
  bosh delete-config --type=runtime --name=dns -n
}

main

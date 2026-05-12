#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  ROOT_DIR=$PWD

  source "${BBL_STATE_DIR}/.envrc"

  env

  bosh upload-release --rebase $ROOT_DIR/candidate-release/*.tgz
  bosh -n upload-stemcell $ROOT_DIR/bosh-stemcell-windows/*.tgz
  bosh -n upload-stemcell $ROOT_DIR/linux-stemcell/*.tgz

  # Need to delete the bosh-dns runtime config because bbl uses a hard-coded
  # bosh-deployment which specifies a bosh-dns version that may conflict with the
  # one we are trying to test.
  bosh delete-config --type=runtime --name=dns -n
}

main

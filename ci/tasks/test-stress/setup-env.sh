#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  source "$PWD/bosh-dns-release/ci/assets/utils.sh"

  export TEST_STRESS_ASSETS=$PWD/bosh-dns-release/ci/assets/test-stress
  export BOSH_DOCKER_CPI_RELEASE_TARBALL="$( echo "$PWD"/bosh-docker-cpi-release/*.tgz )"
  export BOSH_LOG_LEVEL="${BOSH_LOG_LEVEL:-}"

  mkdir -p "${BBL_STATE_DIR}"
  cd "${BBL_STATE_DIR}"

  bbl version
  bbl plan > bbl_plan.txt

  # Customize environment
  cp "$TEST_STRESS_ASSETS"/terraform/* terraform/
  cp "$TEST_STRESS_ASSETS"/director/*.sh .

  bbl --debug up
  bbl print-env > .envrc
  source .envrc

  # Need to delete the bosh-dns runtime config because bbl uses a hard-coded
  # bosh-deployment which specifies a bosh-dns version that may conflict with the
  # one we are trying to test.
  bosh delete-config --type=runtime --name=dns -n
}

main

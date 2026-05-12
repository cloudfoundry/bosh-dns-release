#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  source "${BBL_STATE_DIR}/.envrc"

  cd bosh-dns-release/ci/assets/test-stress/bosh-workspace
  # Run test
  seq 1 "${DEPLOYMENTS_OF_100}" \
    | xargs -n1 -P"${PARALLEL_DEPLOYMENTS}" -I{} \
    -- bosh -d bosh-dns-{} run-errand dns-lookuper
}

main

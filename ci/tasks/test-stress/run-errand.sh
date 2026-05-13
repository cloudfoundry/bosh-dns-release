#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  eval "$(bbl print-env --state-dir="${BBL_STATE_DIR}")"

  cd bosh-dns-release/ci/assets/test-stress/bosh-workspace
  # Run test
  seq 1 "${DEPLOYMENTS_OF_100}" \
    | xargs -n1 -P"${PARALLEL_DEPLOYMENTS}" -I{} \
    -- bosh -d bosh-dns-{} run-errand dns-lookuper
}

main

#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  source "$PWD/bosh-dns-release/ci/assets/utils.sh"

  cd "${BBL_STATE_DIR}"

  bbl version

  eval "$(bbl print-env --state-dir="${BBL_STATE_DIR}")" || echo "error running 'bbl print-env'"
  if ! clean_up_director; then
    echo "Failed to cleanup director"
  fi

  bbl --debug destroy --no-confirm
}

main

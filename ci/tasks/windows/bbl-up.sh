#!/bin/bash
set -eu -o pipefail

BBL_STATE_DIR="${PWD}/${CI_BBL_STATE}"

main() {
  source "$PWD/bosh-dns-release/ci/assets/utils.sh"

  local bosh_release_path=$(echo "$PWD"/bosh-candidate-release/*.tgz)
  local bosh_deployment_dir="$PWD/bosh-deployment"

  mkdir -p "${BBL_STATE_DIR}"
  cd "${BBL_STATE_DIR}"

  bbl version
  bbl plan > bbl_plan.txt

  # bbl 9.0.43 bundles a stale bosh-deployment. Replace it entirely with the
  # pipeline's copy (master) so that stemcell pins, director address, BPM
  # version, and any other drift are all kept current without piecemeal patches.
  rm -rf ./bosh-deployment
  cp -r "${bosh_deployment_dir}" ./bosh-deployment

  # Use the local bosh release
  sed -i "/bosh create-env/a -o \${BBL_STATE_DIR}/bosh-deployment/local-bosh-release-tarball.yml -v local_bosh_release=${bosh_release_path} \\\\" create-director.sh
  # Remove the iam profile ops file - doesn't work with our assume role setup
  sed -i "/iam-instance-profile/d" create-director.sh

  bbl --debug up
  eval "$(bbl print-env --state-dir="${BBL_STATE_DIR}")"
}

main

#!/bin/bash

set -e

main() {
  source "$PWD/bosh-dns-release/ci/assets/utils.sh"
  local output_dir="$PWD/envs-output/"
  local bbl_state_env_repo_dir=$PWD/envs
  trap "commit_bbl_state_dir ${bbl_state_env_repo_dir} ${ENV_NAME} ${output_dir} 'Remove bbl state dir''" EXIT

  (
    source_bbl_env "envs/${ENV_NAME}"
    clean_up_director
  )

  (
    cd "envs/${ENV_NAME}"

    bbl version

    bbl --debug destroy --no-confirm
  )
}

main

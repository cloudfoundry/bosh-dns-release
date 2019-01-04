#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh
  local output_dir="$PWD/cleanup-bbl-state/"
  local bbl_state_env_repo_dir=$PWD/bbl-state
  trap "commit_bbl_state_dir ${bbl_state_env_repo_dir} ${BBL_STATE_DIR} ${output_dir} 'Remove bbl state dir'" EXIT

  if source_bbl_env bbl-state/${BBL_STATE_DIR} && bosh env; then
    clean_up_director docker
  fi

  cd "bbl-state/${BBL_STATE_DIR}"
  bbl version

  bbl --debug destroy --no-confirm
}

main

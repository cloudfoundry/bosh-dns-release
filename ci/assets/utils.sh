#!/bin/bash
set -eu -o pipefail

kill_ssh() {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

source_bbl_env() {
  local bbl_state_dir=${1?'BBL state directory is required.'}

  trap kill_ssh EXIT

  set +x
  eval "$(bbl print-env --state-dir="$bbl_state_dir")"
  set -x
}

commit_bbl_state_dir() {
  local input_dir=${1?'Input git repository absolute path is required.'}
  local bbl_state_dir=${2?'BBL state relative path is required.'}
  local output_dir=${3?'Output git repository absolute path is required.'}
  local commit_message=${4:-'Update bbl state.'}

  pushd "${input_dir}/${bbl_state_dir}"
    if [[ -n $(git status --porcelain) ]]; then
      git config user.name "CI Bot"
      git config user.email "cf-release-integration@pivotal.io"
      git add --all .
      rm -f ../.git/hooks/prepare-commit-msg
      git commit -m "${commit_message}"
    fi
  popd

  shopt -s dotglob
  cp -R "${input_dir}/." "${output_dir}"
}

delete_deployment() {
  local deployment_name="$1"
  set +e
  bosh delete-deployment -d "$deployment_name" -n --force
  set -e
}

clean_up_director() {
  local deployment_name=${1:-""}

  # Ensure the environment is clean
  if [[ -z "$deployment_name" ]]; then
    if [ "$(bosh deployments --column=name | wc -l)" -gt 0 ]; then
      while read -r ds; do
        delete_deployment "$ds"
      done <<< "$(bosh deployments --column=name)"
    fi
  else
    delete_deployment "$deployment_name"
  fi

  # Clean-up old artifacts
  bosh -n clean-up --all
}

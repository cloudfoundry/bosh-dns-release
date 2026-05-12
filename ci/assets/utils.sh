#!/bin/bash
set -eu -o pipefail

delete_deployment() {
  local deployment_name="$1"
  set +e
  bosh delete-deployment -d "$deployment_name" -n --force
  set -e
}

clean_up_director() {
  local deployment_name=${1:-""}

  if ! bosh env; then
    echo 'No director found'
    return
  fi

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

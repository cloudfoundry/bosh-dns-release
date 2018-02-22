#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
state_dir=$PWD/inner-bosh-vars

scripts_directory=$(dirname $0)
pushd ${scripts_directory}
  set +x
  eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"

  # target inner bosh
  export BOSH_CA_CERT="$(bosh int ${state_dir}/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${state_dir}/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x
  pushd inner-bosh-workspace
    # 4. Run test
    seq 1 "${DEPLOYMENTS_OF_100:=10}" \
      | xargs -n1 -P"${PARALLEL_DEPLOYMENTS:=10}" -I{} \
      -- bosh -d bosh-dns-{} run-errand dns-lookuper
  popd
popd

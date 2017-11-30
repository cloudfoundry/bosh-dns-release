#!/bin/bash
set -euxo pipefail

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
export STATE_DIR=$PWD/inner-bosh-vars

scripts_directory=$(dirname $0)
pushd ${scripts_directory}
  set +x
  eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
  set -x

  deployments="${DEPLOYMENTS_OF_100:=10}"

  set +x
  # target inner bosh
  export BOSH_CA_CERT="$(bosh int ${STATE_DIR}/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${STATE_DIR}/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x
  pushd inner-bosh-workspace
    # 4. Run test
    ./check-dns.sh $deployments
  popd
popd

#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

export BOSH_DOCKER_CPI_RELEASE_REPO=$PWD/bosh-docker-cpi-release
export BOSH_DEPLOYMENT_REPO=$PWD/bosh-deployment

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
export STATE_DIR=$(mktemp -d)

scripts_directory=$(dirname $0)
pushd ${scripts_directory}
  set +x
  eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
  set -x

  # 1. Deploy docker hosts to outer director
  pushd docker-hosts-deployment
    ./deploy-docker.sh
  popd

  # 2. Deploy inner director
  pushd inner-bosh-deployment
    ./deploy-director.sh
  popd

  set +x
  # target inner bosh
  export BOSH_CA_CERT="$(bosh int ${STATE_DIR}/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${STATE_DIR}/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x

  bosh env
popd

cp -R ${STATE_DIR}/ inner-bosh-vars/


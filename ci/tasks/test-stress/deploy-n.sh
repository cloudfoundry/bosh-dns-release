#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

deploy_n() {
  deployment_count=$1
  bash ./update-configs.sh
  bosh upload-stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent

  pushd dns-lookuper
    bosh create-release --force --timestamp-version
    bosh upload-release
  popd

  if ! bosh -d bosh-dns-1 deployment > /dev/null ; then
    # docker cpi has a race condition of creating networks on first appearance
    # so, pre-provision the network once on each vm first
    bosh -d bosh-dns-1 deploy -n \
      deployments/bosh-dns.yml \
      -v deployment_name=bosh-dns-1 \
      -v dns_lookuper_release=dns-lookuper \
      -v deployment_count=$deployment_count \
      -v instances=10
  fi

  seq 1 $deployment_count \
    | xargs -n1 -P5 -I{} \
    -- bosh -d bosh-dns-{} deploy -n deployments/bosh-dns.yml \
      -v deployment_name=bosh-dns-{} \
      -v dns_lookuper_release=dns-lookuper \
      -v deployment_count=$deployment_count \
      -v instances=100
}

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
  export BOSH_CA_CERT="$(bosh int ${STATE_DIR}/**/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${STATE_DIR}/**/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x

  pushd inner-bosh-workspace
    # 3. 10x Deploy large bosh-dns deployment to inner director
    deploy_n $deployments
  popd
popd

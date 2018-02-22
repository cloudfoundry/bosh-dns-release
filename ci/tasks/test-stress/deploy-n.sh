#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

deploy_n() {
  local deployment_count=$1
  local parallel_deployments=$2
  local state_dir=$3
  local stemcell_path=$4

  bosh update-cloud-config -n cloud-config.yml

  bosh update-cpi-config -n cpi-config.yml \
    -l ${state_dir}/docker-vars-store.yml \
    -l ../docker-hosts-deployment/vars/docker-vars.yml

  bosh upload-stemcell ${stemcell_path}

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
    | xargs -n1 -P${parallel_deployments} -I{} \
    -- bosh -d bosh-dns-{} deploy -n deployments/bosh-dns.yml \
      -v deployment_name=bosh-dns-{} \
      -v dns_lookuper_release=dns-lookuper \
      -v deployment_count=$deployment_count \
      -v instances=100
}

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
state_dir=$PWD/inner-bosh-vars
stemcell_path=$PWD/stemcell/*.tgz

scripts_directory=$(dirname $0)
pushd ${scripts_directory}
  set +x
  eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
  set -x

  set +x
  # target inner bosh
  export BOSH_CA_CERT="$(bosh int ${state_dir}/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${state_dir}/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x

  pushd inner-bosh-workspace
    # 3. 10x Deploy large bosh-dns deployment to inner director
    deploy_n "${DEPLOYMENTS_OF_100:=10}" "${PARALLEL_DEPLOYMENTS:=3}" "${state_dir}" "${stemcell_path}"
  popd
popd

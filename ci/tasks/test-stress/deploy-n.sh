#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh

  export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
  source_bbl_env $BBL_STATE_DIR

  local state_dir=$PWD/docker-vars
  local stemcell_path=$PWD/stemcell/*.tgz
  local test_stress_assets=$PWD/bosh-dns-release/ci/assets/test-stress/

  # 3. 10x Deploy large bosh-dns deployment to inner director
  deploy_n "${DEPLOYMENTS_OF_100}" "${PARALLEL_DEPLOYMENTS}" "${state_dir}" "${stemcell_path}" "${test_stress_assets}"
}

deploy_n() {
  local deployment_count=$1
  local parallel_deployments=$2
  local state_dir=$3
  local stemcell_path=$4
  local test_stress_assets=$5

  pushd $test_stress_assets/bosh-workspace
    bosh update-config -n --name=docker cloud cloud-config.yml

    bosh update-config -n --name=docker cpi cpi-config.yml \
      -l ${state_dir}/docker-vars-store.yml \
      -l $test_stress_assets/docker-hosts-deployment/vars/docker-vars.yml \
      -l ${BBL_STATE_DIR}/vars/director-vars-file.yml

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
  popd
}

main

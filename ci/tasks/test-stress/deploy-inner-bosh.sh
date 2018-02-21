#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

bosh_docker_cpi_release_repo=$PWD/bosh-docker-cpi-release
bosh_deployment_repo=$PWD/bosh-deployment
stemcell_path=$PWD/stemcell/*.tgz
state_dir=$(mktemp -d)

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}

scripts_directory=$(dirname $0)
pushd ${scripts_directory}
  set +x
  eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
  set -x

  # 1. Deploy docker hosts to outer director
  pushd docker-hosts-deployment
    bbl --state-dir=$BBL_STATE_DIR cloud-config | bosh -n update-cloud-config - \
      -o docker-addressable-zones-template.yml \
      -v jumpbox_table_id=$(bosh int $BBL_STATE_DIR/vars/terraform.tfstate --path /modules/0/resources/aws_route_table.bosh_route_table/primary/id) \
      -v default_table_id=$(bosh int $BBL_STATE_DIR/vars/terraform.tfstate --path /modules/0/resources/aws_route_table.internal_route_table/primary/id)

    bosh upload-stemcell $stemcell_path

    bosh -n deploy -d docker ./deployments/docker.yml \
      -l ./vars/docker-vars.yml \
      --vars-store=${state_dir}/docker-vars-store.yml
  popd

  # 2. Deploy inner director
  pushd inner-bosh-deployment
    bosh upload-stemcell $stemcell_path

    bosh -n -d bosh deploy \
      $bosh_deployment_repo/bosh.yml \
      --vars-store ${state_dir}/vars-store.yml \
      --vars-file  ./vars.yml \
      -o $bosh_deployment_repo/misc/bosh-dev.yml \
      -o ./ops/add-docker-cpi-release.yml \
      -o ./ops/make-stemcell-latest.yml \
      -o ./ops/make-persistent-disk-big.yml \
      -o $bosh_deployment_repo/uaa.yml \
      -o $bosh_deployment_repo/jumpbox-user.yml \
      -o $bosh_deployment_repo/local-dns.yml \
      -o $bosh_deployment_repo/credhub.yml \
      -o ./ops/configure-max-threads.yml \
      -v docker_cpi_release_src_path=$bosh_docker_cpi_release_repo
  popd

  set +x
  # target inner bosh
  export BOSH_CA_CERT="$(bosh int ${state_dir}/vars-store.yml --path /director_ssl/ca)"
  export BOSH_CLIENT="admin"
  export BOSH_CLIENT_SECRET="$(bosh int ${state_dir}/vars-store.yml --path /admin_password)"
  export BOSH_ENVIRONMENT="https://$(bosh int inner-bosh-deployment/vars.yml --path /internal_ip):25555"
  set -x

  bosh env
popd

cp -R ${state_dir}/* inner-bosh-vars/

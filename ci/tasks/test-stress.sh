#!/bin/bash
set -euxo pipefail

deploy_n() {
  deployment_count=$1
  pushd ${scripts_directory}/inner-bosh-workspace
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
  popd
}

export BOSH_DOCKER_CPI_RELEASE_REPO=$PWD/bosh-docker-cpi-release
export BOSH_DEPLOYMENT_REPO=$PWD/bosh-deployment

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
export STATE_DIR=$(mktemp -d)

tasks_directory=$(dirname $0)
scripts_directory=${tasks_directory}/test-stress
cd ${scripts_directory}

set +x
eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
set -x

deployments="${DEPLOYMENTS_OF_100:=10}"

# Ensure the environment is clean
bosh -n -d bosh delete-deployment
bosh -n -d docker delete-deployment

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

  pushd inner-bosh-workspace
    # 3. 10x Deploy large bosh-dns deployment to inner director
    deploy_n $deployments

    # 4. Run test
    ./check-dns.sh $deployments
  popd

set +x
# retarget outer bosh
eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
set -x

# skip teardown of docker container vms since they'll be torn down with the host

# 5. Delete inner director
bosh -n -d bosh delete-deployment

# 6. Delete docker hosts from outer director
bosh -n -d docker delete-deployment

# 7. Clean-up old artifacts
bosh -n clean-up --all

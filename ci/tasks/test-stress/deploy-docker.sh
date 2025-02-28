#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh

  export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}
  source_bbl_env $BBL_STATE_DIR

  local bosh_deployment_repo=$PWD/bosh-deployment
  local docker_release=$(echo $PWD/docker-release/*.tgz)
  local stemcell_path=$PWD/stemcell/*.tgz
  local bosh_dns_release_tarball=$PWD/candidate-release/*.tgz
  local state_dir=$(mktemp -d)

  # Deploy docker hosts to director
  pushd bosh-dns-release/ci/assets/test-stress/docker-hosts-deployment
    bosh -n update-cloud-config "${BBL_STATE_DIR}/cloud-config/cloud-config.yml" \
      -o "${BBL_STATE_DIR}/cloud-config/ops.yml" \
      -o ops/docker-addressable-zones-template.yml \
      --vars-file "${BBL_STATE_DIR}/vars/cloud-config-vars.yml"

    bosh upload-stemcell $stemcell_path
    bosh upload-release $bosh_dns_release_tarball

    bosh -n deploy -d docker deployments/docker.yml \
      -l vars/docker-vars.yml \
      -v director_ip=$(echo $BOSH_ENVIRONMENT | sed 's#https://##g' | sed 's#:.*##g') \
      -v docker_release=$docker_release \
      --vars-store=${state_dir}/docker-vars-store.yml
  popd

  # Copy docker deployment vars to output
  cp -R ${state_dir}/* docker-vars/
}

main

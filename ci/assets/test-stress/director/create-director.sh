#!/bin/sh
bosh int \
  ${BBL_STATE_DIR}/bosh-deployment/bosh.yml \
  --vars-store ${BBL_STATE_DIR}/vars/director-vars-store.yml \
  -l ${BBL_STATE_DIR}/vars/director-vars-file.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/aws/cpi.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/jumpbox-user.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/uaa.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/credhub.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/aws/iam-instance-profile.yml \
  -o ${BBL_STATE_DIR}/bbl-ops-files/aws/bosh-director-ephemeral-ip-ops.yml \
  -o ${BBL_STATE_DIR}/bbl-ops-files/aws/bosh-director-encrypt-disk-ops.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/add-docker-cpi-release.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-max-threads.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-workers.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-pg-max-connections.yml \
  -l ${TEST_STRESS_ASSETS}/director/vars.yml \
  -v docker_cpi_release=$BOSH_DOCKER_CPI_RELEASE_TARBALL \
  > ${BBL_STATE_DIR}/director-manifest.yml

bosh create-env ${BBL_STATE_DIR}/director-manifest.yml \
  --state  ${BBL_STATE_DIR}/vars/bosh-state.json

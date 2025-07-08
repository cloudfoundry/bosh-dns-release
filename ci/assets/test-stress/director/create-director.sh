#!/bin/sh
bosh int \
  ${BBL_STATE_DIR}/bosh-deployment/bosh.yml \
  --vars-store ${BBL_STATE_DIR}/vars/director-vars-store.yml \
  -l ${BBL_STATE_DIR}/vars/director-vars-file.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/aws/cpi.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/jumpbox-user.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/uaa.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/credhub.yml \
  -o ${BBL_STATE_DIR}/bosh-deployment/aws/cpi-assume-role-credentials.yml \
  -o ${BBL_STATE_DIR}/bbl-ops-files/aws/bosh-director-ephemeral-ip-ops.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/add-docker-cpi-release.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-max-threads.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-workers.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/configure-pg-max-connections.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/disable-hm.yml \
  -o ${TEST_STRESS_ASSETS}/director/ops/use-m6-xlarge.yml \
  -l ${TEST_STRESS_ASSETS}/director/vars.yml \
  -v docker_cpi_release=$BOSH_DOCKER_CPI_RELEASE_TARBALL \
  -v access_key_id=$BBL_AWS_ACCESS_KEY_ID \
  -v secret_access_key=$BBL_AWS_SECRET_ACCESS_KEY \
  -v role_arn=$BBL_AWS_ASSUME_ROLE \
  > ${BBL_STATE_DIR}/director-manifest.yml

bosh create-env ${BBL_STATE_DIR}/director-manifest.yml \
  --state  ${BBL_STATE_DIR}/vars/bosh-state.json

#!/bin/bash -eux

bosh upload-stemcell https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent

bosh -n -d bosh deploy \
  $BOSH_DEPLOYMENT_REPO/bosh.yml \
  --vars-store ${STATE_DIR}/vars-store.yml \
  --vars-file  ./vars.yml \
  -o $BOSH_DEPLOYMENT_REPO/misc/bosh-dev.yml \
  -o ./ops/add-docker-cpi-release.yml \
  -o ./ops/make-stemcell-latest.yml \
  -o $BOSH_DEPLOYMENT_REPO/uaa.yml \
  -o $BOSH_DEPLOYMENT_REPO/jumpbox-user.yml \
  -o $BOSH_DEPLOYMENT_REPO/local-dns.yml \
  -o $BOSH_DEPLOYMENT_REPO/credhub.yml \
  -o ./ops/configure-max-threads.yml \
  -v docker_cpi_release_src_path=$BOSH_DOCKER_CPI_RELEASE_REPO

#!/bin/bash
set -euxo pipefail

bosh update-cloud-config -n cloud-config.yml

bosh update-cpi-config -n cpi-config.yml \
  -l ${STATE_DIR}/docker-vars-store.yml \
  -l ../docker-hosts-deployment/vars/docker-vars.yml

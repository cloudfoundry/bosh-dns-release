#!/bin/bash
set -euxo pipefail

bbl --state-dir=$BBL_STATE_DIR cloud-config | bosh -n update-cloud-config - \
  -o docker-addressable-zones-template.yml \
  -v jumpbox_table_id=$(bosh int $BBL_STATE_DIR/vars/terraform.tfstate --path /modules/0/resources/aws_route_table.bosh_route_table/primary/id) \
  -v default_table_id=$(bosh int $BBL_STATE_DIR/vars/terraform.tfstate --path /modules/0/resources/aws_route_table.internal_route_table/primary/id)

bosh upload-stemcell https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent

bosh -n deploy -d docker ./deployments/docker.yml \
  -l ./vars/docker-vars.yml \
  --vars-store=${STATE_DIR}/docker-vars-store.yml

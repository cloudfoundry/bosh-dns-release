#!/bin/bash
set -euxo pipefail

bash ./update-configs.sh
bosh upload-stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent

pushd dns-lookuper
  bosh create-release --force --timestamp-version
  bosh upload-release
popd

deployment_count=$1

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

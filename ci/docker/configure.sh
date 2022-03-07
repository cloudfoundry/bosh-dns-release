#!/bin/bash

set -ex
  lpass status
set +ex

dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-bosh-ecosystem} \
  set-pipeline -p bosh-dns-release-docker \
  -c $dir/pipeline.yml

#!/usr/bin/env bash

set -ex
  lpass status
set +ex

dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-bosh-ecosystem} \
  sp -p bosh-dns-release \
  -c $dir/pipeline.yml

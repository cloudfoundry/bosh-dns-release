#!/usr/bin/env bash

dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-bosh-ecosystem} \
  sp -p bosh-dns-release \
  -c $dir/pipeline.yml

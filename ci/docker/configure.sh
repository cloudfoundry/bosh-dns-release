#!/bin/bash

set -ex
  lpass status
set +ex

dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-production} \
  set-pipeline -p dns-release:docker \
  -c $dir/pipeline.yml \
  --load-vars-from <(lpass show --note "bosh:docker-images concourse secrets")

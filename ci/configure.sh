#!/usr/bin/env bash

set -ex
  lpass status
set +ex

this_dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-production} sp -p bosh-dns-release -c $this_dir/pipeline.yml --load-vars-from <(lpass show 'dns-release pipeline vars' --notes)

#!/usr/bin/env bash

set -ex
  lpass status
set +ex

this_dir=$(dirname $0)

fly -t ${CONCOURSE_TARGET:-production} \
  sp -p bosh-dns-release \
  -c $this_dir/pipeline.yml \
  -l <(lpass show --notes 'dns-release pipeline vars') \
  -l <(lpass show --notes 'tracker-bot-story-delivery') \
  -v "tracker_project_id=2139998"

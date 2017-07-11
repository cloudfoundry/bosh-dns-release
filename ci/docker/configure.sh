#!/bin/bash

exec fly -t production set-pipeline \
  -p dns-release:docker \
  -c ./pipeline.yml \
  --load-vars-from <(lpass show --note "bosh:docker-images concourse secrets")

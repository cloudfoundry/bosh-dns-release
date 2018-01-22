#!/usr/bin/env bash

fly -t production sp -p bosh-dns-release:0.1.x -c ./ci/pipeline.yml --load-vars-from <(lpass show 'dns-release pipeline vars' --notes)

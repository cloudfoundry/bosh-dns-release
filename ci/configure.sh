#!/usr/bin/env bash

fly -t production sp -p dns-release -c ./ci/pipeline.yml --load-vars-from <(lpass show 549774942470095303 --notes)

#!/usr/bin/env bash

fly -t production sp -p dns-release -c ./ci/pipeline.yml

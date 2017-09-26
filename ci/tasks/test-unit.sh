#!/usr/bin/env bash

set -exu

export GOROOT=/usr/local/golang

pushd bosh-dns-release/
  scripts/test-unit
popd

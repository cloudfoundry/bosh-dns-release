#!/usr/bin/env bash

set -exu

pushd bosh-dns-release/
  scripts/test-unit
popd

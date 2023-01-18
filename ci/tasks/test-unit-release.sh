#!/usr/bin/env bash

set -e

pushd bosh-dns-release/
  scripts/test-unit-release
popd

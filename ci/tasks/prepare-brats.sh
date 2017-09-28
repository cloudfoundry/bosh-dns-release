#!/usr/bin/env bash

set -eux

git clone bosh-src bosh-src-output

pinned_version=$(tar -xOzf bosh-release/*.tgz release.MF | grep commit_hash | awk '{ print $2 }' | tr -d '"')

cd bosh-src-output

git checkout $pinned_version

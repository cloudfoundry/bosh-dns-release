#!/usr/bin/env bash

set -eux

pinned_version=$(tar -xOzf bosh-release/*.tgz release.MF | grep commit_hash | awk '{ print $2 }' | tr -d '"')

pushd bosh-src
  # in case $pinned_version commit was made to a release branch instead of master
  git fetch --quiet origin refs/heads/*:refs/remotes/origin/*
popd

git clone bosh-src bosh-src-output

cd bosh-src-output

git checkout $pinned_version

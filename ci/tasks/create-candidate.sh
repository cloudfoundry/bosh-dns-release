#!/usr/bin/env bash

set -e

export ROOT_PATH=$PWD

cd bosh-dns-release

bosh create-release --tarball=../candidate-release/bosh-dns-dev-release.tgz --timestamp-version --force

#!/bin/bash

set -e

git clone bosh-dns-release bumped-bosh-dns-release

export GOPATH=$PWD/bumped-bosh-dns-release

cd bumped-bosh-dns-release/src/bosh-dns

dep ensure -v -update

git status
git add vendor Gopkg.lock
git commit -m "Update vendored dependencies"

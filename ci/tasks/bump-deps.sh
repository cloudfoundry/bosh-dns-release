#!/bin/bash

set -e

git clone bosh-dns-release bumped-bosh-dns-release

export GOPATH=$PWD/bumped-bosh-dns-release

cd bumped-bosh-dns-release/src/bosh-dns

go get -u ./...
go mod tidy
go mod vendor

pushd ../debug
  go get -u ./...
  go mod tidy
  go mod vendor
popd

if [ "$(git status --porcelain)" != "" ]; then
  git status
  git add vendor Gopkg.lock
  git add ../debug/vendor ../debug/Gopkg.lock
  git config user.name "CI Bot"
  git config user.email "cf-bosh-eng@pivotal.io"
  git commit -m "Update vendored dependencies"
fi

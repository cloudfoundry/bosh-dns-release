#!/bin/bash

set -exu


export GOPATH=$PWD/bosh-dns-release
export PATH="${GOPATH}/bin":$PATH
export GIT_SHA="$(cat $GOPATH/.git/HEAD)"

pushd $GOPATH/src/bosh-dns/performance_tests
    go run github.com/onsi/ginkgo/v2/ginkgo -r \
      --randomize-all \
      --randomize-suites \
      --race .
popd

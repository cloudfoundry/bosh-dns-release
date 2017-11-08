#!/bin/bash

set -exu


export GOPATH=$PWD/bosh-dns-release
export PATH="${GOPATH}/bin":$PATH
export GIT_SHA="$(cat $GOPATH/.git/HEAD)"

go install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

pushd $GOPATH/src/bosh-dns/performance_tests
    ginkgo -r -randomizeAllSpecs -randomizeSuites -race .
popd

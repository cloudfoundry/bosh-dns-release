#!/usr/bin/env bash

set -exu

apt-get update && apt-get install -y wget

wget -q -c "https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.1-linux-amd64" -O /usr/local/bin/bosh
echo "f23c3aecec999cfda93a3123ccc724f09856a5c9  /usr/local/bin/bosh" | shasum -c -

chmod +x /usr/local/bin/bosh

export BOSH_INSTALL_TARGET=/usr/local/golang
mkdir -p $BOSH_INSTALL_TARGET

pushd dns-release/
  bosh sync-blobs

  pushd blobs
    ../packages/golang/packaging
  popd
popd

export GOROOT=/usr/local/golang
export GOPATH=$PWD/dns-release
export PATH=$GOPATH/bin:$BOSH_INSTALL_TARGET/bin:$PATH

go install bosh-dns/vendor/github.com/onsi/ginkgo/ginkgo

pushd $GOPATH/src/bosh-dns/dns
    ginkgo -r -randomizeAllSpecs -randomizeSuites -race .
popd

pushd $GOPATH/src/bosh-dns/healthcheck
    ginkgo -r -randomizeAllSpecs -randomizeSuites -race .
popd

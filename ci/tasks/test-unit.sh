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
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$BOSH_INSTALL_TARGET/bin:$PATH

mkdir -p go/src/github.com/cloudfoundry
mkdir -p go/src/github.com/onsi
ln -s $PWD/dns-release $PWD/go/src/github.com/cloudfoundry/dns-release
ln -s $PWD/dns-release/src/vendor/github.com/onsi/ginkgo $PWD/go/src/github.com/onsi/ginkgo

go install github.com/onsi/ginkgo/ginkgo

pushd $GOPATH/src/github.com/cloudfoundry/dns-release/src/dns
    ginkgo -r -randomizeAllSpecs -randomizeSuites -race .
popd

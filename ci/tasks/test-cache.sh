#!/usr/bin/env bash

set -exu

ROOT_DIR=$PWD

export GOPATH=$(mktemp -d)
export COREDNS_PATH=$GOPATH/src/github.com/coredns/coredns
mkdir -p $GOPATH/src/github.com/coredns/coredns
cp -r coredns/* $COREDNS_PATH

cd $COREDNS_PATH
set +e
go get ./...
set -e
go test ./middleware/cache/...


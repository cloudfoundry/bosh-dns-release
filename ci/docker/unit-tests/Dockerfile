FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

ARG GO_VERSION=1.8.3
ARG GO_SHA=1862f4c3d3907e59b04a757cfda0ea7aa9ef39274af99a784f5be843c80c6772
ARG BOSH_VERSION=2.0.36
ARG BOSH_SHA=cc06d8c62891cd7d55812cbac227fe5f8a9e54505cca622bc767c363d7e8cbf6

ENV GO_TAR=go$GO_VERSION.linux-amd64.tar.gz

WORKDIR /usr/local

RUN curl -fSL https://storage.googleapis.com/golang/$GO_TAR -o $GO_TAR \
  && echo $GO_SHA $GO_TAR | sha256sum -c - \
  && tar -xzvf $GO_TAR

RUN curl -fSL https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-$BOSH_VERSION-linux-amd64 -o /usr/bin/bosh \
  && echo $BOSH_SHA /usr/bin/bosh | sha256sum -c - \
  && chmod +x /usr/bin/bosh

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go

FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

WORKDIR /usr/local
RUN ( true \
  && export GO_VERSION=1.7.5 \
  && export GO_SHA256SUM=2e4dd6c44f0693bef4e7b46cc701513d74c3cc44f2419bf519d7868b12931ac3 \
  && curl -L -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz \
  && echo "$GO_SHA256SUM  go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - \
  && tar xzf go${GO_VERSION}.linux-amd64.tar.gz \
)

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go


FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

WORKDIR /usr/local
RUN ( true \
  && export GO_VERSION=1.10.1 \
  && export GO_SHA256SUM=72d820dec546752e5a8303b33b009079c15c2390ce76d67cf514991646c6127b \
  && curl -L -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz \
  && echo "$GO_SHA256SUM  go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - \
  && tar xzf go${GO_VERSION}.linux-amd64.tar.gz \
)

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go


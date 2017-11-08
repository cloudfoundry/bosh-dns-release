FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

WORKDIR /usr/local
RUN ( true \
  && export GO_VERSION=1.8.3 \
  && export GO_SHA256SUM=1862f4c3d3907e59b04a757cfda0ea7aa9ef39274af99a784f5be843c80c6772 \
  && curl -L -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz \
  && echo "$GO_SHA256SUM  go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - \
  && tar xzf go${GO_VERSION}.linux-amd64.tar.gz \
)

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go


FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

WORKDIR /usr/local
RUN ( true \
  && export GO_VERSION=1.9.2 \
  && export GO_SHA256SUM=de874549d9a8d8d8062be05808509c09a88a248e77ec14eb77453530829ac02b \
  && curl -L -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz \
  && echo "$GO_SHA256SUM  go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - \
  && tar xzf go${GO_VERSION}.linux-amd64.tar.gz \
)

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go


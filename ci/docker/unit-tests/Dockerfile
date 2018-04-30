FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

# golang {
ARG GO_VERSION=1.9.2
ARG GO_SHA=de874549d9a8d8d8062be05808509c09a88a248e77ec14eb77453530829ac02b

ENV GO_TAR=go$GO_VERSION.linux-amd64.tar.gz

WORKDIR /usr/local

ENV \
  PATH=/usr/local/go/bin:$PATH \
  GOPATH=/root/go \
  GOROOT=/usr/local/go

RUN curl -fSL https://storage.googleapis.com/golang/$GO_TAR -o $GO_TAR \
  && echo $GO_SHA $GO_TAR | sha256sum -c - \
  && tar -xzf $GO_TAR
# }

# bosh-cli {
ARG BOSH_VERSION=2.0.40
ARG BOSH_SHA=0045668aa6582ddd518cfb2b6134eba439bbb94d

RUN curl -fSL https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-$BOSH_VERSION-linux-amd64 -o /usr/bin/bosh \
  && echo "$BOSH_SHA  /usr/bin/bosh" | shasum -c - \
  && chmod +x /usr/bin/bosh
# }

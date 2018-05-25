FROM ubuntu:14.04

RUN \
  apt-get update \
  && apt-get install -y \
    curl \
  && apt-get clean

COPY --from=golang:1 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

ARG BOSH_VERSION=2.0.40
ARG BOSH_SHA=0045668aa6582ddd518cfb2b6134eba439bbb94d

RUN curl -fSL https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-$BOSH_VERSION-linux-amd64 -o /usr/bin/bosh \
  && echo "$BOSH_SHA  /usr/bin/bosh" | shasum -c - \
  && chmod +x /usr/bin/bosh

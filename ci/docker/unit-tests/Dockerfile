FROM ubuntu:xenial

RUN \
  apt-get update \
  && apt-get install -y \
    build-essential \
    curl \
  && apt-get clean

COPY --from=golang:1 /usr/local/go /usr/local/go
ENV GOROOT=/usr/local/go PATH=/usr/local/go/bin:$PATH

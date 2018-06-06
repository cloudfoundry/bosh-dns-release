FROM ubuntu:trusty
RUN apt-get update && \
  apt-get install -y curl dnsutils git && \
  rm -rf /var/lib/apt/lists/*
RUN curl -fSL "https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-3.0.1-linux-amd64" -o /usr/bin/bosh \
  && echo "58e6853291c3535e77e5128af9f0e8e4303dd57e5a329aa976f197c010517975  /usr/bin/bosh" | sha256sum -c - \
  && chmod +x /usr/bin/bosh

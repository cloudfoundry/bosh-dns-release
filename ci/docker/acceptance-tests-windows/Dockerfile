FROM ubuntu:trusty
RUN apt-get update && \
  apt-get install -y curl dnsutils git && \
  rm -rf /var/lib/apt/lists/*
RUN curl -fSL "https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.28-linux-amd64" -o /usr/bin/bosh \
  && echo "cc06d8c62891cd7d55812cbac227fe5f8a9e54505cca622bc767c363d7e8cbf6  /usr/bin/bosh" | sha256sum -c - \
  && chmod +x /usr/bin/bosh

FROM ubuntu:xenial
RUN apt-get update && \
  apt-get install -y curl dnsutils git && \
  rm -rf /var/lib/apt/lists/*
RUN curl -fSL "https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-6.1.1-linux-amd64" -o /usr/bin/bosh \
  && echo "98936d0bd22db0c13b874294cc1a83014d8074c2577cd2f269297c0099d68381  /usr/bin/bosh" | sha256sum -c - \
  && chmod +x /usr/bin/bosh

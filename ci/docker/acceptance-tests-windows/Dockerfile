FROM ubuntu:trusty
RUN apt-get update && \
  apt-get install -y curl dnsutils git && \
  rm -rf /var/lib/apt/lists/*
RUN curl -fSL "https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.26-linux-amd64" -o /usr/bin/bosh \
  && echo "81673e467a90a22a1365a5981c137b48eee10db8ed2af6adb67191ce35d12c73  /usr/bin/bosh" | sha256sum -c - \
  && chmod +x /usr/bin/bosh

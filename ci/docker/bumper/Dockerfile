FROM ubuntu:xenial

RUN apt-get update && apt-get install -y git && apt-get clean

ADD https://dl.google.com/go/go1.10.1.linux-amd64.tar.gz /tmp
RUN tar -xvzf /tmp/go1.10.1.linux-amd64.tar.gz -C /usr/local/
ENV PATH /usr/local/go/bin:$PATH

ADD https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 /usr/bin/dep
RUN chmod a+x /usr/bin/dep


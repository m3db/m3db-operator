# stage 1: build
FROM golang:1.13-alpine AS builder
LABEL maintainer="The m3db-operator Authors <m3db@googlegroups.com>"

# Install CA certs for curl
RUN apk add --update ca-certificates openssl && \
  rm -rf /var/cache/apk/*

# Install Build Binaries
RUN apk add --update curl git make bash

# Add source code
RUN mkdir -p /go/src/github.com/m3db/m3db-operator
ADD . /go/src/github.com/m3db/m3db-operator

# Build m3dbnode binary
RUN cd /go/src/github.com/m3db/m3db-operator/ && \
    git submodule update --init      && \
    make m3db-operator

# stage 2: lightweight "release"
FROM alpine:latest
LABEL maintainer="The m3db-operator Authors <m3db@googlegroups.com>"

COPY --from=builder /go/src/github.com/m3db/m3db-operator/out/m3db-operator /bin/m3db-operator

ENTRYPOINT [ "/bin/m3db-operator" ]

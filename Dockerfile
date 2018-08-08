FROM alpine

RUN apk add --update ca-certificates openssl && \
  rm -rf /var/cache/apk/*

ADD _output/m3db-operator /usr/local/bin/m3db-operator

CMD ["/bin/sh", "-c", "/usr/local/bin/m3db-operator"]

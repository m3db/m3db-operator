FROM alpine:3.6

RUN adduser -D m3db-operator
USER m3db-operator

ADD build/output/bin/m3db-operator /usr/local/bin/m3db-operator

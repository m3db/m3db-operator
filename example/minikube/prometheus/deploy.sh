#!/bin/bash

source ../minictl.sh

# a work around to a kustomization file until I'll figure out how to source files from a url
minictl apply -f https://raw.githubusercontent.com/coreos/prometheus-operator/v0.40.0/bundle.yaml

## prepare the Prometheus config file for deployment
gzip -fk config/prometheus.yaml

minictl apply -k .

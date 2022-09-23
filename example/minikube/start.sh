#!/bin/bash

# set the resources according to your needs. Too less resource will prevent some containers from functioning
minikube start --cpus 4 --memory 16384 --vm=true
# for now this deployment works with NodePorts but the options to add ingress is here
minikube addons enable ingress
# so that we can use `top` if metrics aren't visible in Grafana
minikube addons enable metrics-server

source minictl.sh

# start working from the script directory
cd "$(dirname "$0")"

# add the monitoring namespace used by Grafana and Prometheus
minictl apply -f monitoring-namespace.yaml

# deploy Grafana
minictl apply -k grafana
# deploy loki
cd loki; ./deploy.sh; cd ..
# deploy Prometheus
cd prometheus; ./deploy.sh; cd ..
# deploy m3db
cd m3db; ./deploy.sh; cd ..

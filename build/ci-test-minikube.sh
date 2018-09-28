#!/usr/bin/env bash

set -xe

sed -i 's#quay.io/m3db/m3db-operator:latest#m3db-operator:local#' manifests/operator.yaml

# TEMP until full e2e suite works
exit

# Apply the required storage for minikube
kubectl apply -f example/storage-fast-minikube.yaml

# JSONPATH provides the conditional values of the a resources metadata
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'

# Create ETCD cluster and wait until the last node (etcd-2) is ready
kubectl apply -f example/etcd-minikube.yaml
ISREADY="etcd-2:Initialized=True;Ready=True;PodScheduled=True"
until kubectl get pods -lapp=etcd -o jsonpath="$JSONPATH" 2>&1 | grep -q "$ISREADY"; do sleep 1; done

# Create M3DB Operator StatefulSet and wait until it's ready
kubectl apply -f manifests/operator.yaml
ISREADY="m3db-operator-0:Initialized=True;Ready=True;PodScheduled=True"
until kubectl -n operator get pods -lname=m3db-operator -o jsonpath="$JSONPATH" 2>&1 | grep -q "$ISREADY"; do sleep 1; done

# Create M3DB cluster and wait until it's ready
kubectl apply -f example/m3db-cluster-minikube.yaml
ISREADY="m3db-cluster-us-west1-a-m3-0:Initialized=True;Ready=True;PodScheduled=True;m3db-cluster-us-west1-b-m3-0:Initialized=True;Ready=True;PodScheduled=True;"
until kubectl get pods -lapp=m3dbnode -o jsonpath="$JSONPATH" 2>&1 | grep -q "$ISREADY"; do  sleep 15; kubectl get all; kubectl describe events; done

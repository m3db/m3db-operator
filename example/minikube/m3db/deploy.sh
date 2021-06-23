#!/bin/bash

source ../minictl.sh

# a work around to a kustomization file until I'll figure out how to source files from a containing folder
minictl apply -f ../../storage-fast-minikube.yaml
minictl apply -f ../../etcd-minikube.yaml
minictl apply -f ../../../bundle.yaml

# wait until operator deployed
while ! minictl api-resources | grep M3DBCluster ; do echo "Waiting for M3DBCluster Kind to be available"; sleep 2; done

# deploy a cluster using the operator
minictl apply -f m3db-cluster.yaml
minictl apply -f service.yaml
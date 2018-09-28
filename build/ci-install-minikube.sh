#!/usr/bin/env bash

set -xe

install_prog(){
  mkdir -p /usr/local/bin/
  curl -Lo $1 $2
  chmod +x $1
  sudo mv $1 /usr/local/bin/
}

docker build -t m3db-operator:local .

# Install required programs
install_prog "kubectl" "https://storage.googleapis.com/kubernetes-release/release/v1.11.2/bin/linux/amd64/kubectl"
install_prog "minikube" "https://github.com/kubernetes/minikube/releases/download/v0.28.0/minikube-linux-amd64"

# Startup minikube
sudo minikube start --vm-driver=none --bootstrapper=localkube --kubernetes-version=v1.10.0  --cpus 4 --memory 6500
sudo minikube update-context
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
until kubectl get nodes -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done

# Ensure storage class is present
sudo minikube addons disable default-storageclass

#!/bin/bash

set -exo pipefail

echo "--- :kubernetes: Installing kind"

KUBE_VERSION=${KUBE_VERSION:-v1.16.15}
KIND_VERSION=${KIND_VERSION:-0.10.0}
CLUSTER_NAME=kind
L_UNAME=$(uname | tr "[:upper:]" "[:lower:]")

mkdir -p "$HOME/bin"
export PATH="$HOME/bin:$PATH"

# Use command -v so that user can have kind in PATH but outside of "$HOME/bin".
if [[ ! $(command -v kind) || "$BUILDKITE" == "true" ]]; then
  curl -sL -o "$HOME/bin/kind" "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-${L_UNAME}-amd64"
  chmod +x "$HOME/bin/kind"
fi

if [[ "$(kind --version)" != "kind version ${KIND_VERSION}" ]]; then
  echo "expected kind version ${KIND_VERSION}, got $(kind --version)"
  exit 1
fi

if [[ ! $(command -v kubectl) || "$BUILDKITE" == "true" ]]; then
  curl -sL -o "$HOME/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/${L_UNAME}/amd64/kubectl"
  chmod +x "$HOME/bin/kubectl"
fi


echo "--- :kubernetes: Deleting existing kind clusters"
kind get clusters -q | while read -r CLUSTER; do
  kind delete cluster --name "$CLUSTER"
done

echo "--- :kubernetes: Creating kind cluster"

# NB(schallert): starting in 1.17, we'll want to move away from the deprecated
# "failure-domain" labels:
# https://kubernetes.io/docs/reference/labels-annotations-taints/#failure-domainbetakubernetesiozone.

cat > cluster.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  kubeadmConfigPatches:
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "failure-domain.beta.kubernetes.io/zone=us-east1-b"
- role: worker
  kubeadmConfigPatches:
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "failure-domain.beta.kubernetes.io/zone=us-east1-c"
- role: worker
  kubeadmConfigPatches:
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "failure-domain.beta.kubernetes.io/zone=us-east1-d"
EOF

# Kind node images are specific to the kind version the cluster is created with:
# https://github.com/kubernetes-sigs/kind/releases
kind create cluster --image "kindest/node:v1.16.15@sha256:c10a63a5bda231c0a379bf91aebf8ad3c79146daca59db816fb963f731852a99" --name "$CLUSTER_NAME" --config cluster.yaml

kubectl get nodes
kubectl cluster-info

echo "--- :kubernetes: Kind cluster created"

echo "--- Waiting for cluster to be ready"
while kubectl get nodes | grep -q NotReady; do
  sleep 5
done

#!/bin/bash

set -exo pipefail

echo "--- :kubernetes: Installing kind"

KUBE_VERSION=${KUBE_VERSION:-v1.15.7}
CLUSTER_NAME=kind
L_UNAME=$(uname | tr "[:upper:]" "[:lower:]")

# mkdir -p "$HOME/bin"

# if [[ ! -x "$HOME/bin/kind" || "$BUILDKITE" == "true" ]]; then
#   curl -sL -o "$HOME/bin/kind" "https://github.com/kubernetes-sigs/kind/releases/download/v0.7.0/kind-${L_UNAME}-amd64"
# fi

# if [[ ! -x "$HOME/bin/kubectl" || "$BUILDKITE" == "true" ]]; then
#   curl -sL -o "$HOME/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/${L_UNAME}/amd64/kubectl"
# fi

# chmod +x "$HOME/bin/kind" "$HOME/bin/kubectl"
# export PATH="$HOME/bin:$PATH"

echo "--- :kubernetes: Deleting existing kind clusters"
kind get clusters -q | while read -r CLUSTER; do
  kind delete cluster --name "$CLUSTER"
done

echo "--- :kubernetes: Creating kind cluster"

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

kind create cluster --image "kindest/node:v1.15.12@sha256:67181f94f0b3072fb56509107b380e38c55e23bf60e6f052fbd8052d26052fb5" --name "$CLUSTER_NAME" --config cluster.yaml

kubectl get nodes
kubectl cluster-info

echo "--- :kubernetes: Kind cluster created"

echo "--- Waiting for cluster to be ready"
while kubectl get nodes | grep -q NotReady; do
  sleep 5
done

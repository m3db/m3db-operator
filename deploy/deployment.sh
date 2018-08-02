#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

kubectl --namespace "${NAMESPACE}" set image deployment/m3db-operator m3db-operator="${IMAGE}:$(git rev-parse head)"
kubectl --namespace "${NAMESPACE}" apply -f deploy/0_namespace.yaml
kubectl --namespace "${NAMESPACE}" apply -f deploy/1_crd.yaml
kubectl --namespace "${NAMESPACE}" apply -f deploy/2_cr.yaml
kubectl --namespace "${NAMESPACE}" apply -f deploy/3_rbac.yaml
kubectl --namespace "${NAMESPACE}" apply -f deploy/4_operator.yaml

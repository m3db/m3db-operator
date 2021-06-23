#!/bin/bash

helm repo add loki https://grafana.github.io/loki/charts
helm repo update
kubectl --context=minikube create namespace loki
helm upgrade --kube-context minikube --install loki --namespace=loki loki/loki
helm upgrade --kube-context minikube --install promtail --namespace=loki loki/promtail --set "loki.serviceName=loki"



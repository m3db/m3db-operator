#!/usr/bin/env bash

set -ex

# M3 Charts 
BUCKET=${2:-"s3://m3-helm-charts-repository/stable/"}
CHART_NAME=${3:-"m3db-operator"}
CHART_DIRECTORY=${4:-"$(pwd)/helm"}
REPO_NAME="${5:-"m3-charts"}"
VERSION=${1:-$(cat helm/${CHART_NAME}/Chart.yaml  | grep "^version" | awk '{print $2}')}

# Helm 
REPO_URL="${BUCKET}/stable/"
HELM_URL=https://storage.googleapis.com/kubernetes-helm
HELM_TARBALL=helm-v2.7.2-linux-amd64.tar.gz

install_helm () {

  # Download and install helm
  curl -o "${HELM_TARBALL}" "${HELM_URL}/${HELM_TARBALL}"
  tar zxvf ${HELM_TARBALL}
  PATH=${PATH}:$(pwd)/linux-amd64/
  export PATH
  rm -f ${HELM_TARBALL}

  # Install helm s3 plugin if not installed
  if [[ $(helm plugin list | grep "^s3") == "" ]]; then
    helm plugin install https://github.com/hypnoglow/helm-s3.git
  fi
}

package_helm () {

  HELM_PACKAGE_DIRECTORY=$(mktemp -d)
  helm init --client-only 
  helm repo add "${REPO_NAME}" "${BUCKET}"

  cd "${CHART_DIRECTORY}/${CHART_NAME}"
  helm package . -d ${HELM_PACKAGE_DIRECTORY}

  helm s3 push "${HELM_PACKAGE_DIRECTORY}/${CHART_NAME}-${VERSION}.tgz" "${REPO_NAME}"
}

install_helm

package_helm


#!/usr/bin/env bash

set -ex

# M3 Charts 
BUCKET="s3://m3-helm-charts-repository/stable/"
CHART_DIRECTORY="$(pwd)/helm"
CHARTS=$(ls -1 ${CHART_DIRECTORY})
REPO_NAME="m3-charts"
HELM_PACKAGE_DIRECTORY=$(mktemp -d)

# Helm 
REPO_URL="${BUCKET}/stable/"
HELM_URL=https://storage.googleapis.com/kubernetes-helm
HELM_TARBALL=helm-v2.7.2-linux-amd64.tar.gz
HELM_EXTRACTED_ARCHIVE="$(pwd)/linux-amd64/"

install_helm () {

  # Download and install helm
  curl -o "${HELM_TARBALL}" "${HELM_URL}/${HELM_TARBALL}"
  tar zxvf ${HELM_TARBALL}
  PATH=${PATH}:${HELM_EXTRACTED_ARCHIVE}
  export PATH
  rm -f ${HELM_TARBALL}

  # Install helm s3 plugin if not installed
  if [[ $(helm plugin list | grep "^s3") == "" ]]; then
    helm plugin install https://github.com/hypnoglow/helm-s3.git
  fi
}

package_helm () {

  for CHART_NAME in ${CHARTS};
  do 
    (
      # Package 
      helm init --client-only 
      helm repo add "${REPO_NAME}" "${BUCKET}"
      helm package "${CHART_DIRECTORY}/${CHART_NAME}" -d ${HELM_PACKAGE_DIRECTORY}
      
      # Push 
      VERSION=$(cat helm/${CHART_NAME}/Chart.yaml  | grep "^version" | awk '{print $2}')
      helm s3 push "${HELM_PACKAGE_DIRECTORY}/${CHART_NAME}-${VERSION}.tgz" "${REPO_NAME}"
    )
  done
}

cleanup () {
  rm -rf ${HELM_PACKAGE_DIRECTORY}
  rm -rf ${HELM_EXTRACTED_ARCHIVE}
}

install_helm

package_helm

cleanup


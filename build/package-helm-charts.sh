#!/usr/bin/env bash

set -ex

# M3 Charts
BUCKET="gs://m3-helm-charts/stable/"
CHART_DIRECTORY="$(pwd)/helm"
CHARTS=( m3db-operator )
REPO_NAME="m3-charts-push"
HELM_PACKAGE_DIRECTORY=$(mktemp -d)
HELMTMP=$(mktemp -d)

function cleanup() {
  rm -rf "${HELM_PACKAGE_DIRECTORY}"
  rm -rf "$HELMTMP"
}

trap cleanup EXIT

# Helm
HELM_URL=https://get.helm.sh/helm-v3.0.2-linux-amd64.tar.gz
HELM_TARBALL="helm.tgz"

install_helm () {
  # Download and install helm
  (
    cd "$HELMTMP"
    wget -q -O $HELM_TARBALL "$HELM_URL"
    tar zxvf $HELM_TARBALL
  )
  PATH="${PATH}:${HELMTMP}/linux-amd64/bin"
  export PATH

  # Install helm gcs plugin if not installed
  if [[ $(helm plugin list | grep "^gcs") == "" ]]; then
    # NB(schallert): You must build and install this locally until the next
    # release is cut.
    helm plugin install https://github.com/hayorov/helm-gcs
  fi
}

package_helm () {
  for CHART_NAME in "${CHARTS[@]}";
  do
    (
      # Package
      helm repo add "${REPO_NAME}" "${BUCKET}"
      helm package "${CHART_DIRECTORY}/${CHART_NAME}" -d "${HELM_PACKAGE_DIRECTORY}"

      # Push
      VERSION=$(grep "^version" "helm/${CHART_NAME}/Chart.yaml" | awk '{print $2}')
      helm gcs push --public "${HELM_PACKAGE_DIRECTORY}/${CHART_NAME}-${VERSION}.tgz" "${REPO_NAME}"
    )
  done
}

if ! command -v helm; then
  install_helm
fi

package_helm

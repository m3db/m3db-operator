#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if ! which go > /dev/null; then
	echo "error: golang needs to be installed"
	exit 1
fi

BIN_DIR="$(pwd)/build/output/bin"
mkdir -p "${BIN_DIR}"
PROJECT_NAME="m3db-operator"
REPO_PATH="github.com/m3db/m3db-operator"
BUILD_PATH="${REPO_PATH}/cmd/${PROJECT_NAME}"
echo "building ${PROJECT_NAME} ..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${BIN_DIR}/${PROJECT_NAME}" "$BUILD_PATH"

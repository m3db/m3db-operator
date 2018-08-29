#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if ! which docker > /dev/null; then
	echo "docker needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. gcr.io/<repo>/<your>-operator"}

GITSHA="$(git rev-parse HEAD)"

echo "building container ${IMAGE}:${GITSHA}..."
docker build -t "${IMAGE}:${GITSHA}" -f Dockerfile .
docker tag "${IMAGE}:${GITSHA}" "${IMAGE}:latest"
docker tag "${IMAGE}:${GITSHA}" "${IMAGE}:${GITSHA}"
docker push "${IMAGE}:${GITSHA}"
docker push "${IMAGE}:latest"

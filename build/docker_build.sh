#!/usr/bin/env bash

if ! which docker > /dev/null; then
	echo "docker needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. gcr.io/<repo>/<your>-operator"}

GITSHA="$(git rev-parse head)"

echo "building container ${IMAGE}:${GITSHA}..."
docker build -t "${IMAGE}:${GITSHA}" -f ./build/Dockerfile .
docker tag "${IMAGE}:${GITSHA}" "${IMAGE}:${GITSHA}"
docker push "${IMAGE}:${GITSHA}"

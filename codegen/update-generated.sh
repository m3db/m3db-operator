#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

vendor/k8s.io/code-generator/generate-groups.sh \
deepcopy \
github.com/m3db/m3db-operator/pkg/generated \
github.com/m3db/m3db-operator/pkg/apis \
m3db:v1alpha1 \
--go-header-file "./codegen/boilerplate.go.txt"

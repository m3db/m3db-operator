#!/bin/bash

set -exuo pipefail

export PACKAGE=github.com/m3db/m3db-operator

echo "--- :git: Updating git submodules"
git submodule update --init --recursive
echo "--- Running unit tests"
make clean-all test-ci-unit lint metalint test-all-gen build-bin

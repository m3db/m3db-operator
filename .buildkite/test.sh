#!/bin/bash

set -exuo pipefail

export PACKAGE=github.com/m3db/m3db-operator

git submodule update --init --recursive
make clean-all test-ci-unit test-all-gen

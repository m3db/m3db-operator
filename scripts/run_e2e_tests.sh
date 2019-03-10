#!/bin/bash

set -ex

function cleanup() {
  pkill -f "kubectl proxy"
}

function main() {
  readonly NAMESPACE="m3db-e2e-test-1"

  command -v kubectl > /dev/null || (echo "error: need kubectl installed" && exit 1)

  # Check that we can reasonably connect to a cluster
  kubectl cluster-info > /dev/null || (echo "unable to fetch cluster info" && exit 1)

  # Delete the namespace in the case it exists
  kubectl delete namespace $NAMESPACE 2> /dev/null || true

  # Wait until it's gone
  local NSCOUNT=0
  while [[ $NSCOUNT -lt 10 ]]; do
    if kubectl get namespace $NAMESPACE 2>&1 | grep -q "not found"; then
      break
    else
      echo "waiting for namespace to be deleted"
      sleep 5
      ((NSCOUNT++))
    fi
  done

  # If a kubectl proxy process is running, delete and wait until it's dead
  if pgrep -fq 'kubectl proxy'; then
    pkill -f 'kubectl proxy'
    local PROXYCOUNT=0
    while [[ $PROXYCOUNT -lt 10 ]]; do
      if pgrep -fq 'kubectl proxy'; then
        echo "waiting for proxy proccess to die"
        sleep 5
        ((PROXYCOUNT++))
      else
        break
      fi
    done
  fi

  trap cleanup EXIT

  kubectl proxy &>/dev/null &
  go clean -testcache
  go test -v -tags integration ./integration/e2e
}

main

#!/bin/bash

set -exo pipefail

if [[ "$1" == "-h" || -z "$ETCD_NS" || -z "$ETCD_POD" || -z "$M3DB_NS" || -z "$M3DB_CLUSTER" ]]; then
  echo "Script for migrating etcd data from m3db-operator 0.2 -> 0.3"
  echo "Usage: ETCD_NS=<namespace> ETCD_POD=<pod> M3DB_NS=<namespace> M3DB_CLUSTER=<cluster_name> ./migrate_etcd_0.2_0.3.sh"
  exit 0
fi

CLUSTER=$M3DB_CLUSTER
NS=$M3DB_NS

if ! kubectl get -n "$NS" m3dbcluster "$CLUSTER" > /dev/null; then
  echo "Could not find m3dbcluster $CLUSTER in namespace $NS"
  exit 1
fi

ENV="$NS/$CLUSTER"
echo "Copying namespace and placement data from env=default_env to env=$ENV"

kubectl exec -n "$ETCD_NS" "$ETCD_POD" -- env ETCDCTL_API=3 etcdctl get _sd.placement/default_env/m3db | kubectl exec -n "$ETCD_NS" "$ETCD_POD" -- env ETCDCTL_API=3 etcdctl put "_sd.placement/$ENV/m3db"
kubectl exec -n "$ETCD_NS" "$ETCD_POD" -- env ETCDCTL_API=3 etcdctl get _kv/default_env/m3db.node.namespaces | kubectl exec -n "$ETCD_NS" "$ETCD_POD" -- env ETCDCTL_API=3 etcdctl put "_kv/$ENV/m3db.node.namespaces"

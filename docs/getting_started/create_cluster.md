# Creating a Cluster

Once you've [installed](installation) the M3DB operator and read over the [requirements](requirements), you can start
creating some M3DB clusters!

## Basic Cluster

**WARNING:** This setup is not intended for production-grade clusters, but rather for "kicking the tires" with the
operator and M3DB. It is intended to work across almost any Kubernetes environment, and as such has as few dependencies
as possible (namely persistent storage). See below for instructions on creating a more durable cluster.

### Etcd

Create an etcd cluster in the same namespace your M3DB cluster will be created in. If you don't have persistent storage
available, this will create a cluster that will not use persistent storage and will likely become unavailable if any of
the pods die:

```
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/v0.1.4/example/etcd/etcd-basic.yaml

# Verify etcd health once pods available
kubectl exec etcd-0 -- env ETCDCTL_API=3 etcdctl endpoint health
# 127.0.0.1:2379 is healthy: successfully committed proposal: took = 2.94668ms
```

If you have remote storage available and would like to jump straight to using it, apply the following manifest for etcd
instead:
```
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/v0.1.4/example/etcd/etcd-pd.yaml
```

### M3DB

Once etcd is available, you can create an M3DB cluster. An example of a very basic M3DB cluster definition is as
follows:
```
apiVersion: operator.m3db.io/v1alpha1
kind: M3DBCluster
metadata:
  name: simple-cluster
spec:
  image: quay.io/m3db/m3dbnode:latest
  replicationFactor: 3
  numberOfShards: 256
  etcdEndpoints:
  - http://etcd-0.etcd:2379
  - http://etcd-1.etcd:2379
  - http://etcd-2.etcd:2379
  isolationGroups:
  - name: group1
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-a>
  - name: group2
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-b>
  - name: group3
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-c>
  podIdentityConfig:
    sources:
      - PodUID
  namespaces:
    - name: metrics-10s:2d
      preset: 10s:2d
```

This will create a highly available cluster with RF=3 spread evenly across the three given zones within a region. A
pod's UID will be used for its [identity][pod-identity]. The cluster will have 1 [namespace](namespace) that stores
metrics for 2 days at 10s resolution.

Next, apply your manifest:
```
$ kubectl apply -f example/simple-cluster.yaml
m3dbcluster.operator.m3db.io/simple-cluster created
```

Shortly after all pods are created you should see the cluster ready!

```
$ kubectl get po -l operator.m3db.io/app=m3db
NAME                    READY   STATUS    RESTARTS   AGE
simple-cluster-rep0-0   1/1     Running   0          1m
simple-cluster-rep1-0   1/1     Running   0          56s
simple-cluster-rep2-0   1/1     Running   0          37s
```

We can verify that the cluster has finished streaming data by peers by checking that an instance has bootstrapped:
```
$ kubectl exec simple-cluster-rep2-0 -- curl -sSf localhost:9002/health
{"ok":true,"status":"up","bootstrapped":true}
```

## Durable Cluster

This is similar to the cluster above, but using persistent volumes. We recommend using [local volumes][local-volumes]
for performance reasons, and since M3DB already replicates your data.

### Etcd

Create an etcd cluster with persistent volumes:
```
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/v0.1.4/example/etcd/etcd-pd.yaml
```

We recommend modifying the `storageClassName` in the manifest to one that matches your cloud provider's fastest remote
storage option, such as `pd-ssd` on GCP.

### M3DB

```
apiVersion: operator.m3db.io/v1alpha1
kind: M3DBCluster
metadata:
  name: persistent-cluster
spec:
  image: quay.io/m3db/m3dbnode:latest
  replicationFactor: 3
  numberOfShards: 256
  isolationGroups:
  - name: group1
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-a>
  - name: group2
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-b>
  - name: group3
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - <zone-c>
  etcdEndpoints:
  - http://etcd-0.etcd:2379
  - http://etcd-1.etcd:2379
  - http://etcd-2.etcd:2379
  podIdentityConfig:
    sources:
      # - NodeName
  namespaces:
    - name: metrics-10s:2d
      preset: 10s:2d
  dataDirVolumeClaimTemplate:
    metadata:
      name: m3db-data
    spec:
      accessModes:
      - ReadWriteOnce
      # storageClassName: local-scsi
      resources:
        requests:
          storage: 350Gi
        limits:
          storage: 350Gi
```

Here we are creating a cluster spread across 3 zones, with each M3DB instance being able to store up to 350gb of data
using your cluster's default storage class.

If you have local disks available, uncomment the two lines to ensure M3DB replaces instances when a pod moves between
hosts, and that the local storage class is used.

## Deleting a Cluster

Delete your M3DB cluster with `kubectl`:
```
kubectl delete m3dbcluster simple-cluster
```

After deleting a cluster you'll have to delete the metadata in etcd if you want to reuse the same etcd cluster for a new
M3DB cluster. On our roadmap is to add a finalizer to `m3dbcluster` API objects to allow the operator to clean up their
data in etcd. Until then, you'll have to do so manually:

**WARNING**: This will delete all data in your etcd cluster. If you are using this cluster for anything other than M3DB,
you will want to delete only the relevant keys (generally anything containing the string `m3db`).

```
kubectl exec etcd-0 -- env ETCDCTL_API=3 etcdctl del --prefix ""
```

[pod-identity]: ../configuration/pod_identity
[local-volumes]: https://kubernetes.io/blog/2018/04/13/local-persistent-volumes-beta/

# M3DB Operator [![e2e](https://travis-ci.org/m3db/m3db-operator.svg?branch=master)](https://travis-ci.org/m3db/m3db-operator)  [![Build status](https://badge.buildkite.com/6cf88054469d7d59a584f618426dc2bd436f816daaf5000db8.svg)](https://buildkite.com/m3/m3db-operator) [![codecov](https://codecov.io/gh/m3db/m3db-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/m3db/m3db-operator)

Project Status: Alpha

The M3DB Operator is a project dedicated to setting up M3DB on Kubernetes. It aims to automate everyday tasks around managing M3DB. Specifically, it aims to automate:

* Creating M3DB clusters
* Destroying M3DB clusters
* Expanding clusters (adding instances)
* Shrinking clusters (removing instances)
* Replacing failed instances

More complete documentation for the project can be found [here][docs].

## Getting Started

The following instructions serve as a quickstart to get an M3DB cluster up and running in your Kubernetes cluster. This
setup is not for production use, as there's no persistent storage. More information on production-grade clusters can be
found in our [docs][docs].

### Kubernetes Cluster Prerequisites

The M3DB operator targets Kubernetes **1.10** and **1.11**. We generally aim to target the latest two minor versions
supported by GKE but welcome community contributions to support more versions!

The M3DB operator is intended for creating highly available clusters across distinct failure domains. For this reason we
currently only support Kubernetes clusters with nodes in at least 3 zones, but [support][zonal] for zonal clusters is
coming soon.

When running on GKE, the user applying the manifests will need the ability to allow `cluster-admin-binding` during the
installation. Use the following `ClusterRoleBinding` with the user name provided by gcloud:

```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=<name@domain.com>
```

### Installing the M3DB Operator

With Helm:

```
helm repo add m3db https://s3.amazonaws.com/m3-helm-charts-repository/stable;
helm install m3db/m3db-operator --namespace m3db-operator
```

With `kubectl`:

```
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/master/bundle.yaml
```

## Managing Clusters

### Creating a Cluster

Create a simple etcd cluster to store M3DB's topology:

```
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/master/example/etcd/etcd-basic.yaml
```

Apply manifest with your zones specified for isolation groups:

```
apiVersion: operator.m3db.io/v1alpha1
kind: M3DBCluster
metadata:
  name: simple-cluster
spec:
  image: quay.io/m3db/m3dbnode:latest
  replicationFactor: 3
  numberOfShards: 256
  isolationGroups:
    - name: <zone-x>
      numInstances: 1
    - name: <zone-y>
      numInstances: 1
    - name: <zone-z>
      numInstances: 1
  podIdentityConfig:
    sources:
      - PodUID
  namespaces:
    - name: metrics-10s:2d
      preset: 10s:2d
```

### Resizing a Cluster

To resize a cluster, specify the new number of instances you want in each zone either by reapplying your manifest or
using `kubectl edit`. The operator will safely scale up or scale down your cluster.

### Deleting a Cluster

Delete a cluster using `kubectl delete`. You will to remove the etcd data as well, or wipe the data generated by the
operator if you intend to reuse the etcd cluster for another M3DB cluster:

```
kubectl exec etcd-0 -- env ETCDCTL_API=3 etcdctl del --keys-only --prefix ""
```

## Contributing

We welcome community contributions to to the M3DB operator! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for more
information. Please note that on creating a pull request you will be asked to agree to the Uber CLA before we can accept
your contribution.

## License
This project is licensed under the Apache license -- see the [LICENSE](https://github.com/m3db/m3db-operator/blob/master/LICENSE) file for details.

[docs]: https://operator.m3db.io/
[zonal]: https://github.com/m3db/m3db-operator/issues/68

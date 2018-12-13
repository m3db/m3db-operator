# M3DB Operator [![e2e](https://travis-ci.org/m3db/m3db-operator.svg?branch=master)](https://travis-ci.org/m3db/m3db-operator)  [![Build status](https://badge.buildkite.com/6cf88054469d7d59a584f618426dc2bd436f816daaf5000db8.svg)](https://buildkite.com/m3/m3db-operator) [![codecov](https://codecov.io/gh/m3db/m3db-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/m3db/m3db-operator)

Project Status: Alpha

The M3DB Operator is a project dedicated to setting up M3DB on Kubernetes. It aims to automate everyday tasks around managing M3DB. Specifically, it aims to automate:

* Creating M3DB clusters
* Destroying M3DB clusters
* Expanding clusters (adding instances)
* Shrinking clusters (removing instances)
* Replacing failed instances


## Getting Started

The following instructions serve as a quickstart to get an M3DB cluster up and running in your Kubernetes cluster.

### Kubernetes Cluster Prerequisites

## Cloud (GKE) Prerequisites
When running on GKE, the user applying the manifests will need the ability to
allow `cluster-admin-binding` during the installation. Use the following
`ClusterRoleBinding` with the user name provided by gcloud:

```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=<name@domain.com>
```

Apply the persistent disk storage resource:

```
kubectl apply -f example/storage-fast-gcp.yaml
```

## etcd Cluster

M3DB stores its topology in etcd, so it is necessary to deploy an etcd cluster for a healthy deployment of M3DB.  

Apply the `etcd` cluster:

```
kubectl apply -f example/etcd.yaml
```

## Installing the M3DB Operator 

With Helm:

```
helm repo add m3db https://s3.amazonaws.com/m3-helm-charts-repository/stable;
helm install m3db/m3db-operator --namespace m3db-operator
```

With `kubectl`:

`kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/master/bundle.yaml`


## Managing Clusters

### Creating a Cluster

Apply the `m3db` cluster:

```
kubectl apply -f https://github.com/m3db/m3db-operator/tree/master/example/etcd/etcd-basic.yaml
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

### Help
```
>make help

  Usage:
    make            <target>

  Targets:
    build-bin       Build m3db-operator binary
    build-docker    Build m3db-operator docker image with go binary
    code-gen        Generate boilerplate code for kubernetes packages
    dep-ensure      Run dep ensure to generate vendor directory
    dep-install     Ensure dep is installed
```

### License
This project is licensed under the Apache license -- see the [LICENSE](https://github.com/m3db/m3db-operator/blob/master/LICENSE) file for details. 

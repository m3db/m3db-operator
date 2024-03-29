# M3 Operator [![Build status](https://badge.buildkite.com/6cf88054469d7d59a584f618426dc2bd436f816daaf5000db8.svg)](https://buildkite.com/m3/m3db-operator) [![codecov](https://codecov.io/gh/m3db/m3db-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/m3db/m3db-operator)

The M3 Operator helps you set up M3 on Kubernetes. It aims to automate everyday tasks around managing M3, specifically, it aims to automate:

-   Creating clusters
-   Destroying clusters
-   Expanding clusters (adding instances)
-   Shrinking clusters (removing instances)
-   Replacing failed instances

## Table of Contents

- [More Information](#more-information)
  - [Community Meetings](#community-meetings)
  - [Office Hours](#office-hours)
- [Install](#install)
  - [Dependencies](#dependencies)
- [Usage](#usage)
  - [Create an etcd Cluster](#create-an-etcd-cluster)
  - [Install the Operator](#install-the-operator)
  - [Create an M3 Cluster](#create-an-m3-cluster)
  - [Resize a Cluster](#resize-a-cluster)
  - [Delete a Cluster](#delete-a-cluster)
- [Contributing](#contributing)

## More Information

-   [Documentation](https://m3db.io/docs/operator/)
-   [Slack](http://bit.ly/m3slack)
-   [Forum (Google Group)](https://groups.google.com/forum/#!forum/m3db)

### Community Meetings

M3 contributors and maintainers have regular meetings. Join our M3 meetup group to receive notifications on upcoming meetings: <https://www.meetup.com/M3-Community/>.

You can find recordings of past meetups here: <https://vimeo.com/user/120001164/folder/2290331>.

### Office Hours

Members of the M3 team hold office hours on the third Thursday of every month from 11-1pm EST. To join, make sure to sign up for a slot here: <https://calendly.com/chronosphere-intro/m3-community-office-hours>.

## Install

### Dependencies

The M3 operator targets Kubernetes **1.21** and higher. We aim to target the latest two minor versions supported by GKE but welcome community contributions to support more versions.

The M3 operator is intended for creating highly available clusters across distinct failure domains. For this reason it only support Kubernetes clusters with nodes **in at least 3 zones**.

## Usage

The following instructions are a quickstart to get a cluster up and running. This setup is not for production use, as it has no persistent storage. [Read the operator documentation](https://m3db.io/docs/operator/) for more information on production-grade clusters.

### Create an etcd Cluster

M3 stores its cluster placements and runtime metadata in [etcd](https://etcd.io/) and needs a running cluster to communicate with.

```shell
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/v0.14.0/example/etcd/etcd-basic.yaml
```

### Install the Operator

Using `kubectl` (installs in the `default` namespace):

```shell
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/v0.14.0/bundle.yaml
```

### Create an M3 Cluster

The following command creates an M3 cluster with 3 replicas of data across 256 shards that connects to the 3 available etcd endpoints.

```shell
kubectl apply -f https://raw.githubusercontent.com/m3db/m3db-operator/master/example/m3db-local.yaml
```

When running on GKE, the user applying the manifests needs the ability to allow `cluster-admin-binding` during installation. Use the following `ClusterRoleBinding` with the user name provided by gcloud:

```shell
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account)
```

### Resize a Cluster

To resize a cluster, specify the new number of instances you want in each zone either by reapplying your manifest or using `kubectl edit`. The operator safely scales up or scales down the cluster.

### Delete a Cluster

```shell
kubectl delete m3dbcluster simple-cluster
```

You also need to remove the etcd data, or wipe the data generated by the operator if you intend to reuse the etcd cluster for another M3 cluster:

```shell
kubectl exec etcd-0 -- env ETCDCTL_API=3 etcdctl del --keys-only --prefix ""
```

## Contributing

You can ask questions and give feedback in the following ways:

-   [Create a GitHub issue](https://github.com/m3db/m3db-operator/issues)
-   [In the public M3 Slack](http://bit.ly/m3slack)
-   [In the M3 forum (Google Group)](https://groups.google.com/forum/#!forum/m3db)

The M3 operator welcomes pull requests, read [the development guide](CONTRIBUTING.md) to help you get setup for building and contributing.

* * *

This project is released under the [Apache License, Version 2.0](https://github.com/m3db/m3/blob/master/LICENSE).

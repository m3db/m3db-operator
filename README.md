# m3db-operator [![e2e](https://travis-ci.org/m3db/m3db-operator.svg?branch=master)](https://travis-ci.org/m3db/m3db-operator)  [![Build status](https://badge.buildkite.com/6cf88054469d7d59a584f618426dc2bd436f816daaf5000db8.svg)](https://buildkite.com/m3/m3db-operator) [![codecov](https://codecov.io/gh/m3db/m3db-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/m3db/m3db-operator)

## Project Status: pre-Alpha

### Kubernetes cluster prerequisites

### GKE
When running on GKE, the user applying the manifests will need the ability to
allow `cluster-admin-binding` during the installation. Use the following
`ClusterRoleBinding` with the user name provided by gcloud

```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=<name@domain.com>
```

Apply the persistent disk storage resource

```
kubectl apply -f example/storage-fast-gcp.yaml
```

## Minikube

Ensure Minikube has enough memory and cpu resources
```
minikube stop && minikube delete && minikube start --cpus 4 --memory 819
```

Apply the persistent disk storage resource

```
kubectl apply -f example/storage-fast-minikube.yaml
```

Label your minikube instance with zone metadata to support scheduling
```
kubectl label nodes minikube failure-domain.beta.kubernetes.io/zone=minikube
```

## Deploy etcd cluster

Apply the `etcd` cluster

```
kubectl apply -f example/etcd.yaml
```

## Deploying the operator

### In-Cluster

This is the primary method of deploying the operator, in which it runs as a StatefulSet inside a Kubernetes cluster and
performs operations using its service account. Permissions are granted via RBAC rules bundled with the operator. Any
services the operator needs to talk to (such as `m3dbnode` + `m3coordinator`) are handled via typical Kubernetes
intra-cluster communication methods.

Generate Go linux binary and push to a Docker registry:

```
make -e IMAGE=<registry>/<repo>/m3db-operator -e LINUX_BUILD=1  build-docker
```

Update the [operator manifest](https://github.com/m3db/m3db-operator/blob/master/manifests/operator.yaml#L93) to include image location:
```
... <snip>
    spec:
      containers:
        - name: m3db-operator
          image: <registry>/<repository>/m3db-operator:<gitsha>
          ports:
          - containerPort: 60000
... <snip>
```

Apply the `m3db-operator` operator

```
kubectl apply -f manifests/operator.yaml
```

### Out-of-Cluster

This method is **only** for development + testing purposes. Production operator deployments should always be in-cluster.

In this manner the operator is running on your local machine, and communicates with the Kubernetes cluster as yourself
(not through a ServiceAccount + RBAC). First, build the operator locally:

```
make build-bin
```

Next, start the operator and point it at your Kubeconfig file so it can communicate with the cluster:
```
$ ./out/m3db-operator -v=2 -logtostderr -kubecfg-file=$HOME/.kube/config -coordinator http://localhost:7201
2018-10-01T16:37:57.443-0400    INFO    constructed a logger
2018-10-01T16:37:57.444-0400    INFO    Go      {"VERSION": "go1.10.3"}
2018-10-01T16:37:57.444-0400    INFO    Go      {"OS": "darwin", "ARCH": "amd64"}
2018-10-01T16:37:57.444-0400    INFO    Operator        {"version": "0.0.1"}
2018-10-01T16:37:57.444-0400    INFO    using OutOfCluster k8s config   {"kubeFile": "/Users/mschalle/.kube/config"}
2018-10-01T16:38:00.405-0400    INFO    processing statefulset  {"name": "etcd"}
2018-10-01T16:38:00.457-0400    INFO    CRD already exists      {"name": "m3dbclusters.operator.m3db.io"}
2018-10-01T16:38:00.457-0400    INFO    found existing  {"clusters": 0}
2018-10-01T16:38:00.457-0400    INFO    starting Operator controller
2018-10-01T16:38:00.457-0400    INFO    waiting for informer caches to sync
2018-10-01T16:38:00.558-0400    INFO    starting workers
2018-10-01T16:38:00.558-0400    INFO    workers started
```

## Managing Clusters

### Creating a Cluster

Apply the `m3db` cluster:

```
kubectl apply -f example/m3db-cluster.yaml
```

**NOTE**: If deploying in-cluster, you'll notice errors indicating that the operator can't communicate with the
coordinator, which it must do in order to create placements and namespaces:
```
2018/10/01 14:27:24 [DEBUG] GET http://localhost:7201/api/v1/namespace
2018/10/01 14:27:24 [ERR] GET http://localhost:7201/api/v1/namespace request failed: Get http://localhost:7201/api/v1/namespace: dial tcp [::1]:7201: connect: connection refused
```

To allow communication between your local operator and the cluster, you can `kubectl port-forward` to the coordinator:
```
$ kubectl port-forward  svc/m3coordinator-m3db-cluster 7201
Forwarding from 127.0.0.1:7201 -> 7201
Forwarding from [::1]:7201 -> 7201
```

The operator will automatically retry operations until it can connect to the coordinator.

### Delete M3 Cluster

Delete M3 Cluster

```
kubectl delete -f example/m3db-cluster.yaml
```

You'll likely want to delete the data in etcd so that if you bring up a new cluster it won't reuse state such as
namespaces and placements from the old one:
```
$ kubectl exec etcd-0 -- env ETCDCTL_API=3 etcdctl del --prefix ""
2
```

Delete M3DB Operator

```
kubectl delete -f manifests/operator.yaml
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

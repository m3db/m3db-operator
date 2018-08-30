# m3db-operator 

## Project Status: pre-Alpha

### Kubernetes cluster prerequisites 


### GKE 
When running on GKE, the user applying the manifests will need the ability to 
allow `cluster-admin-binding` during the installation. Use the following
`ClusterRoleBinding` with the user name provided by gloud 

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

## Deploy etcd cluster

Apply the `etcd` cluster

```
kubectl apply -f example/etcd.yaml
```

### Build and Deploy M3 Cluster

Generate Go linux binary and push to a Docker registry

```
make -e IMAGE=<registry>/<repo>/m3db-operator -e LINUX_BUILD=1  build-docker 
```

Update the [operator manifest](https://github.com/m3db/m3db-operator/blob/master/manifests/operator.yaml#L93) to include image location 
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
Apply the `m3db` cluster

```
kubectl apply -f example/m3db-cluster.yaml
```

### Delete M3 Cluster

Delete M3 Cluster

```
kubectl delete -f example/m3db-cluster.yaml
```

Delete M3DB Operator 

```
kubectrl delete -f manifests/operator.yaml
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



# m3db-operator 

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

### Build and Deploy

Generate Go linux binary and push to a Docker registry

```
make -e IMAGE=<registry>/<repo>/m3db-operator -e LINUX_BUILD=1  build-docker 
```

Apply the `m3db-operator` operator 
```
kubectl apply -f manifests/operator.yaml
```

Apply the `etcd` cluster
```
kubectl apply -f example/etcd.yaml
```

Apply the `m3db` cluster
```
kubectl apply -f example/example-m3db-cluster-gke.yaml
```

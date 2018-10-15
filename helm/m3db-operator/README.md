### Helm Charts for M3DB clusters on Kubernetes

### Prerequisite

[Install helm](https://docs.helm.sh/using_helm/#installing-helm)

### Installing m3db-operator chart

```
cd helm/m3db-operator
helm package . 
helm install m3db-operator-0.0.1.tgz
```


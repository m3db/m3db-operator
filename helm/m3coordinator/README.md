# m3coordinator

TEMP

```
# FROM ROOT OF REPO
helm template ./helm/m3coordinator
```

Works out of the box with a cluster such as:
```
apiVersion: operator.m3db.io/v1alpha1
kind: M3DBCluster
metadata:
  name: test-cluster
spec:
  image: quay.io/m3db/m3dbnode:latest
  replicationFactor: 3
  numberOfShards: 1024
  etcdEndpoints:
  - http://etcd-0.etcd:2379
  - http://etcd-1.etcd:2379
  - http://etcd-2.etcd:2379
  namespaces:
  - name: metrics-2d
    preset: 10s:2d
  - name: metrics-40d
    preset: 1m:4d
  isolationGroups:
  - name: rep0
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - us-east4-a
  - name: rep1
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - us-east4-b
  - name: rep2
    numInstances: 1
    nodeAffinityTerms:
    - key: failure-domain.beta.kubernetes.io/zone
      values:
      - us-east4-c
  podIdentityConfig:
    sources: []
  dataDirVolumeClaimTemplate:
    metadata:
      name: m3db-data
    spec:
      accessModes:
      - ReadWriteOnce
      storageClassName: fast
      resources:
        requests:
          storage: 100Gi
        limits:
          storage: 100Gi
```

# Production Considerations

This doc is here to help guide you in deploying the m3db-operator in a production environment.
Much of what needs to be modified to create a successful production deployment of M3DB is
concentrated in configuration and tuning of M3DB itself, and will highly depend on your
individual requirements and use-cases. That being said, here are some things you should
keep in mind when deploying to production.

## Kubernetes Namespaces

You should deploy all M3DB nodes into their own namespace rather than using "default".
This is especially critical with very large Kubernetes clusters because the m3db-operator
is listening for Pod events in order to tag the M3DB pods with their identity annotation.
When there are many pods, the operator can fall behind trying to sort through so many
events, which can cause the M3DB pods to timeout waiting for their identity, and get
stuck in a restart loop. This can delay the startup of your cluster, or even make it
impossible to restart any pod.

To avoid this, create a new namespace for the M3DBCluster, and then modify the m3db-operator
pod spec with the `-namespace` flag to tell it which namespace M3DB is deployed in. This will
ensure the operator only watches for events in that namespace.

For example, below is a snippet that uses the `-namespace` arg to filter the `m3db-operator`
container to only watch for events in th Kubernetes namespace `production`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app: m3db-operator
    system: m3db
spec:
    spec:
      containers:
      - name: m3db-operator
        args:
        - -namespace
        - production
...
```

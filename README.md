# M3DB Operator [![Build status](https://badge.buildkite.com/6cf88054469d7d59a584f618426dc2bd436f816daaf5000db8.svg)](https://buildkite.com/m3/m3db-operator) [![codecov](https://codecov.io/gh/m3db/m3db-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/m3db/m3db-operator)

The M3DB Operator is a project dedicated to setting up M3DB on Kubernetes. It aims to automate everyday tasks around managing M3DB. Specifically, it aims to automate:

* Creating M3DB clusters
* Destroying M3DB clusters
* Expanding clusters (adding instances)
* Shrinking clusters (removing instances)
* Replacing failed instances

More information:

- [Documentation][docs]
- [Gitter room](https://gitter.im/m3db/kubernetes)
- [M3DB Google Group](https://groups.google.com/forum/#!forum/m3db)

### Kubernetes Cluster Prerequisites

The M3DB operator targets Kubernetes **1.11** and **1.12**. We generally aim to target the latest two minor versions
supported by GKE but welcome community contributions to support more versions!

The M3DB operator is intended for creating highly available clusters across distinct failure domains. For this reason we
currently only support Kubernetes clusters with nodes in at least 3 zones, but [support][zonal] for zonal clusters is
coming soon.

When running on GKE, the user applying the manifests will need the ability to allow `cluster-admin-binding` during the
installation. Use the following `ClusterRoleBinding` with the user name provided by gcloud:

```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account)
```

## Contributing

We welcome community contributions to to the M3DB operator! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for more
information. Please note that on creating a pull request you will be asked to agree to the Uber CLA before we can accept
your contribution.

## License
This project is licensed under the Apache license -- see the [LICENSE](https://github.com/m3db/m3db-operator/blob/master/LICENSE) file for details.

[docs]: https://m3db.io/docs/operator/
[zonal]: https://github.com/m3db/m3db-operator/issues/68

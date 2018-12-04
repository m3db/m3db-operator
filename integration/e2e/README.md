# E2E Tests

This directory contains the framework for testing the M3DB Operator against "real" Kubernetes clusters. It tests basic
functionality as well as operator behavior under various failure scenarios.

Design of this framework is influenced by CoreOS's excellent [etcd-operator](https://github.com/coreos/etcd-operator)
and [prometheus-operator](https://github.com/coreos/prometheus-operator) e2e tests.

## WARNING!

This test framework will attempt to only make modifications to namespaces it creates which start with the prefix
`m3db-e2e-test`. It will attempt to clean those namespaces up at the end of test runs.

**HOWEVER**, to test failure scenarios the framework may have to modify resources outside of its namespaces (such as
persistent volumes). For this reason, it is not recommended to run this test suite against production clusters. In the
future we will add config parameters to not modify objects outside of our namespaces, but it is still not recommended to
run these tests against production clusters.

Scripts are included for creating clusters on GKE which would be safe to test against in isolation.

## Running the tests

**NOTE**: If you are running on a platform such as GKE you may need to grant your user the ability to create roles. From
the [GKE
docs](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#prerequisites_for_using_role-based_access_control):
```
kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole cluster-admin --user $USER_ACCOUNT
```

It is generally a good idea to run the tests a clean go test cache, otherwise running the tests with the same test code
may cause cached tests to run. The make target accomplishes this by running `go clean -testcache` first.

```
# With a working Kubernetes cluster and a correctly configured current kubectl context:
$ make test-e2e
--- test-e2e
go test -v -tags integration ./integration/e2e
2018-11-22T15:36:41.370-0500    INFO    setting up test suite
2018-11-22T15:36:44.206-0500    INFO    creating namespace      {"namespace": "m3db-e2e-test-1"}
...
```

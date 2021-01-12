# Changelog

## 0.12.1

0.12.1 fixes bugs related to cluster startup and update due to the release of M3DB 1.0.

* [ENHANCEMENT] Wait for replica to update pods before processing next statefulset update ([#260][260])
* [BUGFIX] Ensure fresh cluster startup succeeds in M3 1.0+ ([#261][261])

## 0.12.0

0.12.0 adds support for running sidecar containers in the M3DB pods and ensures
clusters created by the operator use 1.0 M3DB configs by default.

### Breaking Changes

0.12.0 updates the default configuration file for M3DB. This is a breaking
change as the operator will not be able to create a working default
configuration file for pre-v1.0.0 deployments of M3DB. To use the new version
of the operator with an older version of m3db you will have to provide a custom
ConfigMap.

* [FEATURE] Support adding sidecar containers to M3DB pods. ([#253][253])
* [ENHANCEMENT] Update default configuration file for M3DB. ([#250][250])

## 0.11.0

0.11.0 ensures the operator is compatible with clusters running M3 1.0. It removes usage of M3 APIs that were deprecated
as part of that release.

* [FEATURE] Allow AggregationOptions to be set for a namespace. ([#248][248])
* [ENHANCEMENT] Update use of now deleted namespace urls in operator ([#247][247])
* [ENHANCEMENT] Add calls to /namespace/ready if supported by coordinator ([#245][245])


## 0.10.0

0.10.0 adds initial support for safe, graceful cluster upgrades. See [the upgrade
docs](https://operator.m3db.io/getting_started/update_cluster/) for more info.

This release also includes documentation enhancements, and adds the `coldWritesEnabled` field  on namespaces to allow
enabling M3DB cold writes.

Finally, this release includes two new API fields to make it easier for users to manage their clusters:

1. A new `freeze` field on clusters allows a user to stop all operations on a cluster while potentially performing
   manual changes.

2. The new `externalCoordinator.serviceEndpoint` field allows controlling the cluster via a coordinator in another
   namespace, allowing users to have a single coordinator responsible for serving m3admin APIs for all clusters across
   any namespace.
   - **WARNING**: The minumum required M3 version to use with this field is `v0.15.9`, which includes a fix for managing
     namespaces in environments other than that for which the coordinator is provisioned.

### Breaking Changes

0.10.0 makes the default pod management policy for StatefulSets created by the operator
[`Parallel`](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#pod-management-policies). This
should be transparent to most users, but means that when new pods are added to a cluster, or pods are manually deleted,
they will all be recreated at once rather than after each has bootstrapped. This should lead to faster upgrades and
cluster resizing operations.

Change set:

* [FEATURE] Allow setting static external coordinator ([#242][242])
* [FEATURE] Add the `coldWritesEnabled` option to Namespace options ([#233][233])
* [ENHANCEMENT] Default Parallel pod management ([#230][230])
* [FEATURE] Add frozen field to cluster spec that will suspend changes ([#241][241])
* [DOCS] Add documentation on updating a cluster ([#240][240])
* [ENHANCEMENT] Always remove update annotation after processing StatefulSet ([#237][237])
* [ENHANCEMENT] Ignore replicas when checking for an update ([#238][238])
* [MISC] Create constant for annotation value indicating enabled ([#239][239])
* [MISC] Implement placement Set API ([#234][234])
* [FEATURE] Add logic to update StatefulSets ([#236][236])
* [MISC] Fix bug in TestHandleUpdateClusterCreatesStatefulSets ([#235][235])
* [DOCS] Update helm install docs ([#231][231])

## 0.9.0

0.9.0 includes support for attaching a custom Kubernetes service account to M3DB pods (enabling use of
PodSecurityPolicies and the like), and an improvement in how new StatefulSets are created when others are unhealthy.

* [FEATURE] Support custom svc account for M3DB pods ([#225][225])
* [ENHANCEMENT] Create missing statefulsets before waiting for ready ([#227][227])

## 0.8.0

0.8.0 includes changes to improve operator performance and reduce load on Kubernetes API servers. The operator will only
watch Pods and StatefulSets with a non-empty `operator.m3db.io/app` label (included on every StatefulSet the operator
generates). Additionally the operator will not unnecessarily update a cluster's Status if there is no change. The
operator now uses Kubernetes client v0.17.2.

* [ENHANCEMENT] Only list objects created by operator ([#222][222])
* [MISC] Update kubernetes client to v0.17.2 ([#221][221])
* [MISC] Update ci-scripts ([#220][220])
* [ENHANCEMENT] Don't update Status if noop ([#219][219])

## 0.7.0

0.7.0 includes changes to allow an M3DB cluster to be administered with a coordinator external to the cluster. It also
supports passing annotations to pod templates, experimental support for using the `Parallel` pod management policy on
M3DB StatefulSets, and support for InitContainers.

* [FEATURE] Break ext coord into separate config ([#216][216])
* [FEATURE] Support Parallel pod management policies. ([#211][211])
* [FEATURE] Added initial support for PodMetaData, handling Annotations only ([#210][210])
* [FEATURE] Support custom InitContainers in cluster spec ([#209][209])
* [FEATURE] Support an external controlling coordinator ([#208][208])


## 0.6.0

0.6.0 includes a fix to allow M3DB nodes to receive traffic while bootstrapping, and an option to limit what namespaces
the operator watches resources in, which should greatly help users running the operator in massive Kubernetes clusters.

* [ENHANCEMENT] Ensure dbnodes in DNS when bootstrapping ([#206][206])
* [FEATURE] Add one option able to only watch one specific namespace ([#205][205])


## 0.5.0

0.5.0 includes a bug fix for passing cluster annotations to pods, as well as a backwards-compatible addition of a new
base environment variable `M3CLUSTER_ENVIRONMENT` which contains the `${NAMESPACE}/${CLUSTER_NAME}`-formatted variable
used for the cluster's environment in etcd.

* [FEATURE] Expose m3cluster env in pod spec ([#197][197])
* [BUGFIX] Apply annotations to pods in created StatefulSets ([#196][196])

## 0.4.0

0.4.0 includes minor feature additions that won't have any change in behavior for existing users.

* [FEATURE] Support custom env vars in cluster spec ([#194][194])
* [ENHANCEMENT] Add topic client ([#190][190])
* [ENHANCEMENT] Migrate to Go modules ([#188][188])
* [FEATURE] Allow overriding node endpoint format ([#183][183])

## 0.3.0

0.3.0 is focused on some behind the scenes reliability improvements. Changes such as using purpose-build M3DB health
endpoints, using `PATCH` to do partial updates to non-operator owned resources, and giving M3DB pods `SYS_RESOURCE` by
default should make operated clusters work in more environments with no changes.

Users that have had etcd-related issues when deleting and recreating M3DB clusters will also be happy, as by default the
operator will delete the metadata associated with an M3DB cluster from etcd when a cluster is deleted. Users can set
`keepEtcdDataOnDelete` to `true` on their cluster specs to disable this behavior.

* [ENHNACEMENT] Use Kubernetes 1.14 libraries ([#167][167])
* [ENHANCEMENT] Add SYS_RESOURCE if security context not set ([#147][147])
* [BUGFIX] Use patch instead of update for resources not owned by operator ([#162][162])
* [ENHANCEMENT] Add HTTP JSONPB request method to client and update callers ([#163][163])
* [ENHANCEMENT] Support image pull secrets ([#160][160])
* [FEATURE] Add carbon ingester port config to cluster spec ([#158][158])
* [FEATURE] Support custom annotations ([#155][155])
* [ENHANCEMENT] Always create missing stateful sets ([#148][148])
* [ENHANCEMENT] Use dbnode health/bootstrap endpoints ([#135][135])
* [FEATURE] Clear data in etcd on cluster delete ([#154][154]) ([#181][181])
* [ENHANCEMENT] Continuously reconcile operator CRD ([#149][149])
* [ENHANCEMENT] Use CRD status subresource ([#152][152])
* [DOCS] Update 0.2.0 breaking changes ([#146][146])
* [ENHANCEMENT] Add better error messages for time parsing from yaml for namespaces ([#144][144])
* [BUGFIX] Fix 0.2.0 migration script ([#143][143])
* [DOCS] Include prometheus monitoring instructions ([#140][140])

## 0.2.0

The theme of this release is usability improvements and more granular control over node placement.

Features such as specifying etcd endpoints directly on the cluster spec eliminate the need to provide a manual
configuration for custom etcd endpoints. Per-cluster etcd environments will allow users to collocate multiple m3db
clusters on a single etcd cluster.

Users can now specify more complex affinity terms, and specify taints that their cluster tolerates to allow dedicating
specific nodes to M3DB. See the [affinity docs][affinity-docs] for more.

* [FEATURE] Allow specifying of etcd endpoints on M3DBCluster spec ([#99][99])
* [FEATURE] Allow specifying security contexts for M3DB pods ([#107][107])
* [FEATURE] Allow specifying tolerations of M3DB pods ([#111][111])
* [FEATURE] Allow specifying pod priority classes ([#119][119])
* [FEATURE] Use a dedicated etcd-environment per-cluster to support sharing etcd clusters ([#99][99])
* [FEATURE] Support more granular node affinity per-isolation group ([#106][106]) ([#131][131])
* [ENHANCEMENT] Change default M3DB bootstrapper config to recover more easily when an entire cluster is taken down
  ([#112][112])
* [ENHANCEMENT] Build + release with Go 1.12 ([#114][114])
* [ENHANCEMENT] Continuously reconcile configmaps ([#118][118])
* [BUGFIX] Allow unknown protobuf fields to be unmarshalled ([#117][117])
* [BUGFIX] Fix pod removal when removing more than 1 pod at a time ([#125][125])

### Breaking Changes

#### API Changes

0.2.0 includes breaking changes to the way isolation groups are defined. In 1.x.x, the `name` of an isolation group was
assumed to be the value of `failure-domain.beta.kubernetes.io/zone` unless a separate key was manually specified. In
0.2.0 we chose to require more explicit definition of isolation groups to allow more complex affinity requirements, as
are described in the [example docs][affinity-docs].

An example of an old isolation group to pin to the zone `us-west1-b` might look like this:

```yaml
isolationGroups:
- name: us-west1-b
  numInstances: 3
  ...
```

In the new API this must be formatted as
```yaml
isolationGroups:
- name: group1 # can be any name you like
  numInstances: 3
  nodeAffinityTerms:
  - key: failure-domain.beta.kubernetes.io/zone
    values:
    - us-west1-b
```


#### Etcd Keys

0.2.0 changes how M3DB stores its cluster topology in etcd to allow for multiple M3DB clusters to share an etcd cluster.
A [migration script][etcd-migrate] is provided to copy etcd data from the old format to the new format. If migrating an
operated cluster, run that script (see script for instructions) and then rolling restart your M3DB pods by deleting them
one at a time.

If using a custom configmap, this same change will require a modification to your configmap. See the
[warning][configmap-warning] in the docs about how to ensure your configmap is compatible.

## 0.1.4

* [FEATURE] Added the ability to use a specific StorageClass per-isolation group (StatefulSet) for clusters without
  topology aware volume provisioning ([#98][98])
* [BUGFIX] Fixed a bug where pods were incorrectly selected if the cluster had labels ([#100][100])

## 0.1.3

* [BUGFIX] Fix a bug in parsing custom namespace durations ([#94][94]).
* [BUGFIX] Ensure m3cordinator API errors errors are checked correctly ([#97][97])

## 0.1.2

* Update default cluster ConfigMap to include parameters required by latest M3DB.
* Add event `patch` permission to default RBAC role.

## 0.1.1

* Fix helm manifests.

## 0.1.0

* Initial release.

[affinity-docs]: https://operator.m3db.io/configuration/node_affinity/
[etcd-migrate]: https://github.com/m3db/m3db-operator/blob/master/scripts/migrate_etcd_0.1_0.2.sh
[configmap-warning]: https://operator.m3db.io/configuration/configuring_m3db/#environment-warning

[94]: https://github.com/m3db/m3db-operator/pull/94
[97]: https://github.com/m3db/m3db-operator/pull/97
[98]: https://github.com/m3db/m3db-operator/pull/98
[99]: https://github.com/m3db/m3db-operator/pull/99
[100]: https://github.com/m3db/m3db-operator/pull/100
[106]: https://github.com/m3db/m3db-operator/pull/106
[107]: https://github.com/m3db/m3db-operator/pull/107
[111]: https://github.com/m3db/m3db-operator/pull/111
[112]: https://github.com/m3db/m3db-operator/pull/112
[114]: https://github.com/m3db/m3db-operator/pull/114
[117]: https://github.com/m3db/m3db-operator/pull/117
[118]: https://github.com/m3db/m3db-operator/pull/118
[119]: https://github.com/m3db/m3db-operator/pull/119
[125]: https://github.com/m3db/m3db-operator/pull/125
[131]: https://github.com/m3db/m3db-operator/pull/131
[135]: https://github.com/m3db/m3db-operator/pull/135
[140]: https://github.com/m3db/m3db-operator/pull/140
[141]: https://github.com/m3db/m3db-operator/pull/141
[143]: https://github.com/m3db/m3db-operator/pull/143
[144]: https://github.com/m3db/m3db-operator/pull/144
[146]: https://github.com/m3db/m3db-operator/pull/146
[147]: https://github.com/m3db/m3db-operator/pull/147
[148]: https://github.com/m3db/m3db-operator/pull/148
[149]: https://github.com/m3db/m3db-operator/pull/149
[150]: https://github.com/m3db/m3db-operator/pull/150
[152]: https://github.com/m3db/m3db-operator/pull/152
[154]: https://github.com/m3db/m3db-operator/pull/154
[155]: https://github.com/m3db/m3db-operator/pull/155
[158]: https://github.com/m3db/m3db-operator/pull/158
[160]: https://github.com/m3db/m3db-operator/pull/160
[162]: https://github.com/m3db/m3db-operator/pull/162
[163]: https://github.com/m3db/m3db-operator/pull/163
[167]: https://github.com/m3db/m3db-operator/pull/167
[169]: https://github.com/m3db/m3db-operator/pull/169
[181]: https://github.com/m3db/m3db-operator/pull/181
[183]: https://github.com/m3db/m3db-operator/pull/183
[188]: https://github.com/m3db/m3db-operator/pull/188
[190]: https://github.com/m3db/m3db-operator/pull/190
[194]: https://github.com/m3db/m3db-operator/pull/194
[196]: https://github.com/m3db/m3db-operator/pull/196
[197]: https://github.com/m3db/m3db-operator/pull/197
[205]: https://github.com/m3db/m3db-operator/pull/205
[206]: https://github.com/m3db/m3db-operator/pull/206
[208]: https://github.com/m3db/m3db-operator/pull/208
[209]: https://github.com/m3db/m3db-operator/pull/209
[210]: https://github.com/m3db/m3db-operator/pull/210
[211]: https://github.com/m3db/m3db-operator/pull/211
[216]: https://github.com/m3db/m3db-operator/pull/216
[219]: https://github.com/m3db/m3db-operator/pull/219
[220]: https://github.com/m3db/m3db-operator/pull/220
[221]: https://github.com/m3db/m3db-operator/pull/221
[222]: https://github.com/m3db/m3db-operator/pull/222
[225]: https://github.com/m3db/m3db-operator/pull/225
[227]: https://github.com/m3db/m3db-operator/pull/227
[231]: https://github.com/m3db/m3db-operator/pull/231
[234]: https://github.com/m3db/m3db-operator/pull/234
[235]: https://github.com/m3db/m3db-operator/pull/235
[236]: https://github.com/m3db/m3db-operator/pull/236
[237]: https://github.com/m3db/m3db-operator/pull/237
[238]: https://github.com/m3db/m3db-operator/pull/238
[239]: https://github.com/m3db/m3db-operator/pull/239
[240]: https://github.com/m3db/m3db-operator/pull/240
[241]: https://github.com/m3db/m3db-operator/pull/241
[230]: https://github.com/m3db/m3db-operator/pull/230
[233]: https://github.com/m3db/m3db-operator/pull/233
[242]: https://github.com/m3db/m3db-operator/pull/242
[245]: https://github.com/m3db/m3db-operator/pull/245
[247]: https://github.com/m3db/m3db-operator/pull/247
[248]: https://github.com/m3db/m3db-operator/pull/248
[250]: https://github.com/m3db/m3db-operator/pull/250
[253]: https://github.com/m3db/m3db-operator/pull/253
[260]: https://github.com/m3db/m3db-operator/pull/260
[261]: https://github.com/m3db/m3db-operator/pull/261

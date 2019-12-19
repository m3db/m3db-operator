# Changelog

## 0.3.0

0.3.0 is focused on some behind the scenes reliability improvements. Changes such as using purpose-build M3DB health
endpoints, using `PATCH` to do partial updates to non-operator owned resources, and giving M3DB pods `SYS_RESOURCE` by
default should make operated clusters work in more environments with no changes.

Users that have had etcd-related issues when deleting and recreating M3DB clusters will also be happy, as by default the
opeator will delete the metadata associated with an M3DB cluster from etcd when a cluster is deleted. Users can set
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

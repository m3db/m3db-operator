# Changelog

## 0.2.0

TODO: High-level notes

* [FEATURE] Allow specifying of etcd endpoints on M3DBCluster spec ([#99][99])
* [FEATURE] Allow specifying security contexts for M3DB pods ([#107][107])
* [FEATURE] Allow specifying tolerations of m3db pods ([#111][111])
* [FEATURE] Allow specifying pod priority classes ([#119][119])
* [FEATURE] Use a dedicated etcd-environment per-cluster to support sharing etcd clusters ([#99][99])
* [FEATURE] Support more granular node affinity per-isolation group ([#106][106])
* [ENHANCEMENT] Change default M3DB bootstrapper config to recover more easily when an entire cluster is taken down
  ([#112][112])
* [ENHANCEMENT] Build + release with Go 1.12 ([#114][114])
* [ENHANCEMENT] Continuously reconcile configmaps ([#118][118])
* [BUGFIX] Allow unknown protobuf fields to be unmarshalled ([#117][117])

## 0.1.4

* [FEATURE] Added the ability to use a specific StorageClass per-isolation group (StatefulSet) for clusters without
  topology aware volume provisioning ([#98][98])
* [BUGFIX] Fixed a bug where pods were incorrectly selected if the cluster had labels ([#100][100])

## 0.1.3

* [BUGFIX] Fix a bug in parsing custom namespace durations ([#94][94]).
* [BUGFIX] Ensure m3cordinator API errors errors are checked correctly ([#97][97])

## 0.1.2

* Update default cluster ConfigMap to include parameters required by latest m3db.
* Add event `patch` permission to default RBAC role.

## 0.1.0

* Fix helm manifests.

## 0.1.0

* Initial release.

[94]: https://github.com/m3db/m3db-operator/pull/94
[97]: https://github.com/m3db/m3db-operator/pull/97
[98]: https://github.com/m3db/m3db-operator/pull/98
[100]: https://github.com/m3db/m3db-operator/pull/100
[106]: https://github.com/m3db/m3db-operator/pull/106
[107]: https://github.com/m3db/m3db-operator/pull/107
[111]: https://github.com/m3db/m3db-operator/pull/111
[112]: https://github.com/m3db/m3db-operator/pull/112
[114]: https://github.com/m3db/m3db-operator/pull/114
[117]: https://github.com/m3db/m3db-operator/pull/117
[118]: https://github.com/m3db/m3db-operator/pull/118
[119]: https://github.com/m3db/m3db-operator/pull/119
[99]: https://github.com/m3db/m3db-operator/pull/99

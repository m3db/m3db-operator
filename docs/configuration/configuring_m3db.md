# Configuring M3DB

By default the operator will apply a configmap with basic M3DB options and settings for the coordinator to direct
Prometheus reads/writes to the cluster. This template can be found
[here](https://github.com/m3db/m3db-operator/blob/master/assets/default-config.tmpl).

To apply custom a configuration for the M3DB cluster, one can set the `configMapName` parameter of the cluster [spec] to
an existing configmap.

[spec]: ../api

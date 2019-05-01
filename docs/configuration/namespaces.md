# Namespaces

M3DB uses the concept of [namespaces][m3db-namespaces] to determine how metrics are stored and retained. The M3DB
operator allows a user to define their own namespaces, or to use a set of presets we consider to be suitable for
production use cases.

Namespaces are configured as part of an `m3dbcluster` [spec][api-namespaces].

## Presets

### `10s:2d`

This preset will store metrics at 10 second resolution for 2 days. For example, in your cluster spec:

```yaml
spec:
...
  namespaces:
  - name: metrics-short-term
    preset: 10s:2d
```

### `1m:40d`

This preset will store metrics at 1 minute resolution for 40 days.

```yaml
spec:
...
  namespaces:
  - name: metrics-long-term
    preset: 1m:40d
```

## Custom Namespaces

You can also define your own custom namespaces by setting the `NamespaceOptions` within a cluster spec. The
[API][api-ns-options] lists all available fields. As an example, a namespace to store 7 days of data may look like:
```yaml
...
spec:
...
  namespaces:
  - name: custom-7d
    options:
      bootstrapEnabled: true
      flushEnabled: true
      writesToCommitLog: true
      cleanupEnabled: true
      snapshotEnabled: true
      repairEnabled: false
      retentionOptions:
        retentionPeriodDuration: 168h
        blockSizeDuration: 12h
        bufferFutureDuration: 20m
        bufferPastDuration: 20m
        blockDataExpiry: true
        blockDataExpiryAfterNotAccessPeriodDuration: 5m
      indexOptions:
        enabled: true
        blockSizeDuration: 12h
```


[api-namespaces]: ../api#namespace
[api-ns-options]: ../api#namespaceoptions
[m3db-namespaces]: https://docs.m3db.io/operational_guide/namespace_configuration/

## Device Injector Plugin

This sample plugin can inject devices and mounts into containers using
pod annotations.

### Device Annotations

Devices are annotated using the `devices.nri.io` annotation key prefix.
The key `devices.nri.io/container.$CONTAINER_NAME` annotates devices to
be injected into `$CONTAINER_NAME`. The keys `devices.nri.io` and
`devices.nri.io/pod` annotate devices to be injected into all containers.

The annotation syntax for device injection is

```
- path: /dev/dev0
  type: {c|b}
  major: 1
  minor: 3
  file_mode: <permission mode>
  uid: <user ID>
  gid: <group ID>
- path: /dev/dev1
  ...
```

`file_mode`, `uid` and `gid` can be omitted, the rest are mandatory.

### Mount Annotations

Mounts are annotated in a similar manner to devices, but using the
`mounts.nri.io` annotation key prefix. The annotation syntax for mount
injection is

```
  - source: <mount source0>
    destination: <mount destination0>
    type: <mount type0>
    options:
      - option0
        option1
        ...
  - source: <mount source1>
    ...
```

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled. Start the plugin on the target node
(`device-injector -idx 10`), create a pod with some annotated devices or
mounts, then verify that those get injected to the containers according
to the annotations. See the [sample pod spec](sample-device-inject.yaml)
for an example.
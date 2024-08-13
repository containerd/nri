## Device Injector Plugin

This sample plugin can inject Linux device nodes, CDI devices, and mounts into
containers using pod annotations.

### Device Annotations

Devices are annotated using the `devices.nri.io` annotation key prefix.
The key `devices.nri.io/container.$CONTAINER_NAME` annotates devices to
be injected into `$CONTAINER_NAME`. The keys `devices.nri.io` and
`devices.nri.io/pod` annotate devices to be injected into containers
without any other, container-specific device annotations. Only one of
these latter two annotations will be ever taken into account. If both are
present, `devices.nri.io/pod` is used and `devices.nri.io` is silently
ignored, otherwise `devices.nri.io`, in the absence of additional suffix text,  is processed as shorthand for the `devices.nri.io/pod` annotation. The order of precedence is `devices.nri.io/container.$CONTAINER_NAME` is used, unless not present, then `devices.nri.io/pod` followed by the `devices.nri.io` shorthand annotation.

The annotation value syntax for device injection is

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

### CDI Device Annotations

CDI devices are annotated in a similar manner to devices, but using the
`cdi-devices.nri.io` annotation key prefix. The annotation value for CDI
devices is the list of CDI device names to inject.

For instance, the following annotation

```
metadata:
  name: bash
  annotations:
    cdi-devices.nri.io/container.c0: |
      - vendor0.com/device=null
    cdi-devices.nri.io/container.c1: |
      - vendor0.com/device=zero
    cdi-devices.nri.io/container.c2: |
      - vendor0.com/device=dev0
      - vendor1.com/device=dev0
      - vendor1.com/device=dev1
    cdi-devices.nri.io/container.mgmt: |
      - vendor0.com/device=all
```

requests the injection of the CDI device vendor0.com/device=null to container
c0, the injection of the CDI device vendor0.com/device=zero to container c1,
the injection of the CDI devices vendor0.com/device=dev0, vendor1.com/device=dev0
and vendor1.com/device=dev1 to container c2, and the injection of the CDI device
vendor0.com/device=all to container mgmt.

### Mount Annotations

Mounts are annotated in a similar manner to devices, but using the
`mounts.nri.io` annotation key prefix. The annotation value syntax for mount
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
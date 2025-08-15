## Device Injector Plugin

This sample plugin can inject Linux device nodes, CDI devices, and mounts into
containers using pod annotations.

### Device Annotations

Devices are annotated using the `devices.noderesource.dev` annotation key
prefix. The key `devices.noderesource.dev/container.$CONTAINER_NAME` annotates
devices to be injected into `$CONTAINER_NAME`. The keys
`devices.noderesource.dev` and `devices.noderesource.dev/pod` annotate devices
to be injected into containers without any other, container-specific device
annotations. Only one of these latter two annotations will be ever taken into
account. If both are present, `devices.noderesource.dev/pod` is used and
`devices.noderesource.dev` is silently ignored, otherwise
`devices.noderesource.dev`, in the absence of additional suffix text, is
processed as shorthand for the `devices.noderesource.dev/pod` annotation.

For compatibility with older versions of this plugin, the prefix
`devices.nri.io` is also supported and follows identical rules. When
equivalently-specific annotations are present with both the
`devices.noderesource.dev` prefix and the `devices.nri.io` prefix, the
annotation with the `devices.noderesource.dev` prefix is used and
`devices.nri.io` is silently ignored. In all cases, the most-specific
annotation is preferred regardless of prefix.

The order of precedence is as follows:

1. `devices.noderesource.dev/container.$CONTAINER_NAME`
2. `devices.nri.io/container.$CONTAINER_NAME`
3. `devices.noderesource.dev/pod`
4. `devices.nri.io/pod`
5. `devices.noderesource.dev`
6. `devices.nri.io`

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
`cdi-devices.noderesource.dev` annotation key prefix. As with devices, the
`cdi-devices.nri.io` annotation key prefix is also supported.

The annotation value for CDI devices is the list of CDI device names to inject.

For instance, the following annotation

```
metadata:
  name: bash
  annotations:
    cdi-devices.noderesource.dev/container.c0: |
      - vendor0.com/device=null
    cdi-devices.noderesource.dev/container.c1: |
      - vendor0.com/device=zero
    cdi-devices.noderesource.dev/container.c2: |
      - vendor0.com/device=dev0
      - vendor1.com/device=dev0
      - vendor1.com/device=dev1
    cdi-devices.noderesource.dev/container.mgmt: |
      - vendor0.com/device=all
```

requests the injection of the CDI device `vendor0.com/device=null` to container
c0, the injection of the CDI device `vendor0.com/device=zero` to container c1,
the injection of the CDI devices `vendor0.com/device=dev0`,
`vendor1.com/device=dev0` and `vendor1.com/device=dev1` to container c2, and
the injection of the CDI device `vendor0.com/device=all` to container mgmt.

### Mount Annotations

Mounts are annotated in a similar manner to devices, but using the
`mounts.noderesource.dev` annotation key prefix. As with devices, the
`mounts.nri.io` annotation key prefix is also supported.

The annotation value syntax for mount injection is

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

## Deployment

The NRI repository contains minimal kustomize overlays for this plugin at
[contrib/kustomize/device-injector](../../contrib/kustomize/device-injector).

Deploy the latest release with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/device-injector
```

Deploy a specific release with:

```bash
RELEASE_TAG=v0.10.0
kubectl apply -k "github.com/containerd/nri/contrib/kustomize/device-injector?ref=${RELEASE_TAG}"
```

Deploy the latest development build from tip of the main branch with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/device-injector/unstable
```

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled. Start the plugin on the target node
(`device-injector -idx 10`), create a pod with some annotated devices or
mounts, then verify that those get injected to the containers according
to the annotations. See the [sample pod spec](sample-device-inject.yaml)
for an example.

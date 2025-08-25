## RDT Plugin

This sample plugin can adjust RDT configuration of containers using pod
annotations.

> [CAUTION]
> This plugin is experimental and only intended for demonstration purposes.

### Annotations

Pod annotations can be used to adjust the RDT configuration of containers. See
[IntelRdt of runtime-spec](https://github.com/opencontainers/runtime-spec/blob/main/config-linux.md#intelrdt)
for details.

| Annotation Key                                                    | Format                             | Description                                                         |
|-------------------------------------------------------------------|------------------------------------|---------------------------------------------------------------------|
| `closid.rdt.noderesource.dev/container.$CONTAINER_NAME`           | string                             | Set the `intelRdt.closID` of the container configuration.           |
| `schemata.rdt.noderesource.dev/container.$CONTAINER_NAME`         | array of strings, separated by `,` | Set the `intelRdt.schemata` of the container configuration.         |
| `enablemonitoring.rdt.noderesource.dev/container.$CONTAINER_NAME` | bool                               | Set the `intelRdt.enableMonitoring` of the container configuration. |

See [sample pod spec](sample-rdt-adjust.yaml) for a practical example.

## Deployment

The NRI repository contains minimal kustomize overlays for this plugin at
[contrib/kustomize/rdt](../../contrib/kustomize/rdt).

Deploy the latest release with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/rdt
```

Deploy a specific release with:

```bash
RELEASE_TAG=v0.11.0
kubectl apply -k "github.com/containerd/nri/contrib/kustomize/rdt?ref=${RELEASE_TAG}"
```

Deploy the latest development build from tip of the main branch with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/rdt/unstable
```

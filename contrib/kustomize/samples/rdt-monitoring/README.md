# Per-container RDT monitoring with NRI hook-injector

This example demonstrates how to enable per-container RDT monitoring using OCI
hooks. It works as a bridge-gap solution until native support
(`linux.intelRdt.enableMonitoring` of OCI runtime-spec) is available.

References:

- https://github.com/opencontainers/runtime-spec/blob/main/config-linux.md#intelrdt
- https://github.com/opencontainers/runc/pull/4832
- https://github.com/containerd/nri/pull/215

## Design

The sample leverages the [hook-injector](../../../../plugins/hook-injector)
sample plugin to inject OCI hooks that manage the per-container monitoring
groups. It deploys a DaemonSet and consists of the following parts:

1. A custom location on the host (/etc/containers/nri/rdt-hook) is used to
   store the OCI hook binary and configuration.
2. An init container creates the OCI hook binary on the host.
3. A second init container creates the OCI hook configuration on the host.
4. The hook-injector NRI plugin is run in the main container. The hook injector
   is configured to only watch for OCI hooks in the custom location.
5. The hook is injected into all containers created after this.

> [!NOTE] The setup enables RDT monitoring for all new containers on the node.
> The hook is injected into every container and the hook binary does not
> provide any means to skip creation of the per-container monitoring group. In
> most scenarios this shouldn't be a problem as the number of available
> monitoring groups is high (in the order of hundreds).

## Deployment

Deploy the sample kustomize overlay:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/samples/rdt-monitoring
```

The functionality can be verified by creating a new pod and checking the
`/sys/fs/resctrl/<container-id>` directory on the node where the pod is
deployed. An example:

```bash
$ kubectl run --image registry.k8s.io/pause rdt-test

$ kubectl get pod rdt-test -o go-template='{{.spec.nodeName}}{{"\n"}}{{(index .status.containerStatuses 0).containerID}}{{"\n"}}'
node-1
containerd://477ba96b86e0f7756790d5c52ffa94feab0028f0785a23d143fd9390f09e35f6
```

Then check the node:

```bash
$ ssh node-1

$ cat /sys/fs/resctrl/mon_groups/477ba96b86e0f7756790d5c52ffa94feab0028f0785a23d143fd9390f09e35f6/tasks
1817117
1817149
1817150
1817151
```

## OCI Hook Injector Plugin

The [OCI runtime configuration](https://github.com/opencontainers/runtime-spec/blob/main/spec.md)
supports [hooks](https://github.com/opencontainers/runtime-spec/blob/main/config.md#posix-platform-hooks), which are custom actions related to the lifecycle of a container.
This plugins performs OCI hook injection into containers using
the [Hook Manager](https://github.com/containers/podman/tree/main/pkg/hooks)
package of [podman](https://github.com/containers/podman).
[CRI-O](https://github.com/cri-o/cri-o) has native hook injection support using
the same package. This plugin essentially achieves CRI-O compatible OCI hook
injection for other runtimes using NRI.

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled. Start the plugin on the target node
(`hook-injector -idx 10`), put a suitable [OCI hook configuration](https://github.com/containers/podman/blob/main/pkg/hooks/docs/oci-hooks.5.md)
in place, then create a pod with matching containers and check the results.

You can use the [sample hook configuration](etc/containers/oci/hooks.d), the
[sample hook](usr/local/sbin/demo-hook.sh) and the [sample pod spec](sample-hook-inject.yaml)
to test this.

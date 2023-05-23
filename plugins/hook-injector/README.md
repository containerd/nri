## OCI Hook Injector Plugin

The [OCI runtime configuration](https://github.com/opencontainers/runtime-spec/blob/main/spec.md) supports [hooks](https://github.com/opencontainers/runtime-spec/blob/main/config.md#posix-platform-hooks), which are custom actions related to the lifecycle of a container. This plugin performs OCI hook injection into containers using the [Hook Manager](https://github.com/containers/podman/tree/8bcc086b1b9d8aa0ef3bb08d37542adf9de26ac5/pkg/hooks) package of [podman](https://github.com/containers/podman). [CRI-O](https://github.com/cri-o/cri-o) has native hook injection support using the same package. This plugin essentially achieves CRI-O compatible OCI hook injection for other runtimes using NRI.

## Testing

You can test this plugin using a Kubernetes cluster/node with a container runtime that has NRI support enabled ([Enabling NRI in Containerd](https://github.com/containerd/containerd/blob/main/docs/NRI.md#enabling-nri-support-in-containerd)). Once you've enabled NRI on your runtime, you can use the sample hook configuration, placing it at `/etc/containers/oci/hooks.d`, and the [sample hook](usr/local/sbin/demo-hook.sh), placing it at `/usr/local/sbin/`.

>*Note:* OCI hook configuration details and default file paths can be found in the [OCI Configuration Package Documentation](https://pkg.go.dev/github.com/containers/podman/v3/pkg/hooks)

Start the plugin directly on the target node by running `hook-injector -idx 10` from the folder containing the binary. Alternatively, you can create a symbolic link to the hook-injector binary in the plugin path configured for the runtime, with the idx as the prefix (ex. `10-hook-injector`)

Additional details on hook configuration can be found in the [OCI hook configuration](https://github.com/containers/podman/blob/8bcc086b1b9d8aa0ef3bb08d37542adf9de26ac5/pkg/hooks/docs/oci-hooks.5.md) documentation.

Finally, create a test pod using the [sample pod spec](sample-hook-inject.yaml) and check for the log output of the hook, which will be at `/tmp/demo-hook.log`
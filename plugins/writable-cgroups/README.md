# Writable Cgroups NRI Plugin (Experimental)

This is an experimental NRI plugin designed to safely enable writable cgroups
(`/sys/fs/cgroup`) inside containers.

## Purpose & Context

This plugin serves as a test-bed for validating the behavior and security model
of "delegated cgroup management" in Kubernetes environments. It was developed in
response to [KEP-5474](https://github.com/kubernetes/enhancements/issues/5474),
which originally proposed adding explicit API support for writable cgroups.

However, the evolving consensus is to move away from API additions and instead
leverage the Linux kernel's
[`nsdelegate` mount option](https://man7.org/linux/man-pages/man7/cgroups.7.html#:~:text=Cgroups%20v2%20delegation%3A%20nsdelegate%20and%20cgroup%20namespaces)
directly within container runtimes. This approach allows runtimes to
automatically provide safe, writable cgroup access when the host is correctly
configured, removing the need for user-facing API changes.

For a detailed design rationale and decision log, please see the
[Delegated Cgroup Management Design Document](https://docs.google.com/document/d/1MJZADe-_fO95wwolUvrxGcm6rhWZRgti__7mjlu-dV8/edit?usp=sharing).

## How It Works

This plugin intercepts container creation requests and checks for the presence
of the `nsdelegate` mount option on the host's root cgroup hierarchy.

1. **Safety Check:** On startup, the plugin inspects the host's mount table (via
   `/host/proc/1/mountinfo` by default). It verifies that the `cgroup2`
   filesystem is mounted with the `nsdelegate` option.
2. **Conditional Activation:**
    * If `nsdelegate` is **absent** on the host: The plugin logs a warning and
      takes **no action**, ensuring safety. Containers retain the default
      Read-Only cgroup mount.
    * If `nsdelegate` is **present** on the host: The plugin proceeds to check
      for enabling annotations.
3. **Enabling:** If the safety check passes AND a container is annotated, the
plugin modifies the container spec to mount `/sys/fs/cgroup` as **Read-Write
(`rw`)** instead of Read-Only (`ro`).

### Why is this safe?

When `nsdelegate` is enabled on the host, the kernel enforces strict boundaries
at the cgroup namespace level. Even with a Read-Write mount, a container:

* **Cannot** modify its own resource limits (e.g., `memory.max`) set by the
  runtime (writes are denied with `EPERM`).
* **Can** create sub-cgroups and manage resources for its own child processes.

## Usage

### Prerequisites

* A container runtime with NRI support enabled.
* The host system must have cgroup v2 enabled and mounted with the `nsdelegate`
  option.

### Deployment

Deploy the plugin binary to your node and ensure it is registered with the NRI
service.

**Command Line Arguments:**

* `-idx`: Plugin index.
* `-socket-path`: Path to the NRI socket.
* `-host-mount-file`: Path to the host's mountinfo file (default:
  `/host/proc/1/mountinfo`). Ensure this file is accessible to the plugin (e.g.,
  via a bind mount in a DaemonSet).

### Annotations

To enable writable cgroups for a workload, add the following annotation to your
Pod:

**Pod-Level (Applies to all containers):**

```yaml
annotations:
  cgroups.noderesource.dev/writable: "true"
```

**Container-Level (Applies to a specific container):**

```yaml
annotations:
  cgroups.noderesource.dev/writable.container.<container_name>: "true"
```

## Status

This plugin is **experimental** and intended for testing and validation purposes
only. It is not recommended for production use until the behavior is
standardized in upstream container runtimes.
